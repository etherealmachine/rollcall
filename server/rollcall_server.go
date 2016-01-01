package main

import (
	"math/rand"
	"net"
	"sort"
	"time"

	"github.com/twinj/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	rpb "github.com/etherealmachine/rollcall/proto"
)

const (
	port         = ":50051"
	tickInterval = time.Second
)

type AccountServer struct {
	accounts map[string]*rpb.Account
}

func NewAccountServer() *AccountServer {
	return &AccountServer{
		accounts: make(map[string]*rpb.Account),
	}
}

func (s *AccountServer) RegisterAccount(ctx context.Context, req *rpb.RegisterAccountRequest) (*rpb.RegisterAccountReply, error) {
	acct := req.Account
	acct.Id = uuid.NewV4().String()
	s.accounts[acct.Id] = acct
	return &rpb.RegisterAccountReply{
		AccountId: acct.Id,
	}, nil
}

func (s *AccountServer) GetAccount(ctx context.Context, req *rpb.GetAccountRequest) (*rpb.GetAccountReply, error) {
	return &rpb.GetAccountReply{
		Account: s.accounts[req.AccountId],
	}, nil
}

func (s *AccountServer) processTransactions(listener <-chan []*rpb.Transaction) {
	for lst := range listener {
		for _, t := range lst {
			buyer := s.accounts[t.BuyerAccountId]
			seller := s.accounts[t.SellerAccountId]
			buyer.Balance -= t.Price * t.Quantity
			buyer.Holdings += t.Quantity
			seller.Balance += t.Price * t.Quantity
			seller.Holdings -= t.Quantity
		}
	}
}

type MarketServer struct {
	tick      uint64
	ticker    <-chan time.Time
	bids      BidQueue
	asks      AskQueue
	listeners chan chan<- []*rpb.Transaction
}

func NewMarketServer() *MarketServer {
	return &MarketServer{
		ticker:    time.Tick(tickInterval),
		listeners: make(chan chan<- []*rpb.Transaction, 10000),
	}
}

type BidQueue []*rpb.Order

func (q BidQueue) Len() int      { return len(q) }
func (q BidQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q BidQueue) Less(i, j int) bool {
	if q[i].Price == q[j].Price {
		return rand.Float64() < 0.5
	}
	return q[i].Price > q[j].Price
}

type AskQueue []*rpb.Order

func (q AskQueue) Len() int      { return len(q) }
func (q AskQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q AskQueue) Less(i, j int) bool {
	if q[i].Price == q[j].Price {
		return rand.Float64() < 0.5
	}
	return q[i].Price < q[j].Price
}

func (s *MarketServer) processTicks() {
	for _ = range s.ticker {
		grpclog.Printf("tick %d: %d bids, %d asks", s.tick, len(s.bids), len(s.asks))
		sort.Sort(s.bids)
		sort.Sort(s.asks)
		var priceSum uint64
		var transactions []*rpb.Transaction
		for len(s.bids) > 0 && len(s.asks) > 0 {
			bid, ask := s.bids[0], s.asks[0]
			if bid.Expiration > 0 && s.tick >= bid.Expiration {
				s.bids = s.bids[1:]
				continue
			}
			if ask.Expiration > 0 && s.tick >= ask.Expiration {
				s.asks = s.asks[1:]
				continue
			}
			if bid.Price < ask.Price {
				break
			}
			t := &rpb.Transaction{
				BuyOrderId:      bid.Id,
				SellOrderId:     ask.Id,
				BuyerAccountId:  bid.AccountId,
				SellerAccountId: ask.AccountId,
				Tick:            s.tick,
			}
			if bid.Quantity > ask.Quantity {
				t.Quantity = ask.Quantity
				bid.Quantity -= ask.Quantity
				s.asks = s.asks[1:]
			} else if bid.Quantity < ask.Quantity {
				t.Quantity = bid.Quantity
				ask.Quantity -= bid.Quantity
				s.bids = s.bids[1:]
			} else {
				t.Quantity = ask.Quantity
				s.asks = s.asks[1:]
				s.bids = s.bids[1:]
			}
			priceSum += bid.Price + ask.Price
			transactions = append(transactions, t)
		}
		price := uint64(float64(priceSum) / float64(len(transactions)*2))
		for _, t := range transactions {
			t.Price = price
		}
		listeners := s.broadcast(transactions)
		grpclog.Printf("tick %d: broadcasted %d transactions to %d listeners", s.tick, len(transactions), listeners)
		s.tick++
	}
}

func (s *MarketServer) broadcast(transactions []*rpb.Transaction) int {
	var listeners []chan<- []*rpb.Transaction
	for {
		select {
		case c := <-s.listeners:
			listeners = append(listeners, c)
			c <- transactions
		default:
			for _, l := range listeners {
				s.listeners <- l
			}
			return len(listeners)
		}
	}
}

func (s *MarketServer) PutOrder(ctx context.Context, req *rpb.PutOrderRequest) (*rpb.PutOrderReply, error) {
	order := req.Order
	order.Id = uuid.NewV4().String()
	switch order.OrderType {
	case rpb.Order_BID:
		s.bids = append(s.bids, order)
	case rpb.Order_ASK:
		s.asks = append(s.asks, order)
	}
	return &rpb.PutOrderReply{
		OrderId: order.Id,
	}, nil
}

func (s *MarketServer) Transactions(req *rpb.TransactionsRequest, stream rpb.MarketService_TransactionsServer) error {
	listener := make(chan []*rpb.Transaction, 10000)
	defer close(listener)
	s.listeners <- listener
	for lst := range listener {
		for _, t := range lst {
			if err := stream.Send(t); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	l, err := net.Listen("tcp", port)
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	rpcServer := grpc.NewServer()
	acctServer := NewAccountServer()
	rpb.RegisterAccountServiceServer(rpcServer, acctServer)
	mktServer := NewMarketServer()
	rpb.RegisterMarketServiceServer(rpcServer, mktServer)
	go mktServer.processTicks()
	listener := make(chan []*rpb.Transaction)
	mktServer.listeners <- listener
	go acctServer.processTransactions(listener)
	grpclog.Printf("server listening on %s", port)
	if err := rpcServer.Serve(l); err != nil {
		grpclog.Fatal(err)
	}
}
