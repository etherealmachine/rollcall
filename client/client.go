package main

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	rpb "github.com/etherealmachine/rollcall/proto"
)

func mustParseUint64(s string) uint64 {
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		grpclog.Fatalf("error parsing uint64 from %s: %v", s, err)
	}
	return i
}

func watchTransactions(ctx context.Context, mktService rpb.MarketServiceClient) {
	c, err := mktService.Transactions(ctx, &rpb.TransactionsRequest{})
	if err != nil {
		grpclog.Fatalf("error watching transactions: %v", err)
	}
	for {
		t, err := c.Recv()
		if err == io.EOF {
			return
		} else if err != nil {
			if err != nil {
				grpclog.Fatalf("error watching transactions: %v", err)
			}
		}
		grpclog.Println(t)
	}
}

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		grpclog.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	ctx := context.Background()
	acctService := rpb.NewAccountServiceClient(conn)
	mktService := rpb.NewMarketServiceClient(conn)

	bs, err := os.Open("accounts.csv")
	if err != nil {
		grpclog.Fatalf("error reading accounts file: %v", err)
	}
	r := csv.NewReader(bs)
	rows, err := r.ReadAll()
	if err != nil {
		grpclog.Fatalf("error reading accounts file: %v", err)
	}
	var accts []*rpb.Account
	for i, row := range rows {
		if i == 0 {
			continue
		}
		acct := &rpb.Account{
			Balance:  mustParseUint64(row[0]),
			Holdings: mustParseUint64(row[1]),
		}
		reply, err := acctService.RegisterAccount(ctx, &rpb.RegisterAccountRequest{
			Account: acct,
		})
		if err != nil {
			grpclog.Fatalf("error registering account on row %d: %v", i, err)
		}
		acct.Id = reply.AccountId
		accts = append(accts, acct)
	}

	go watchTransactions(ctx, mktService)

	bs, err = os.Open("orders.csv")
	if err != nil {
		grpclog.Fatalf("error reading orders file: %v", err)
	}
	r = csv.NewReader(bs)
	rows, err = r.ReadAll()
	if err != nil {
		grpclog.Fatalf("error reading orders file: %v", err)
	}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		delay := mustParseUint64(row[0])
		order := &rpb.Order{
			AccountId:  accts[mustParseUint64(row[1])].Id,
			Price:      mustParseUint64(row[2]),
			Quantity:   mustParseUint64(row[3]),
			Expiration: mustParseUint64(row[4]),
		}
		switch row[5] {
		case "bid":
			order.OrderType = rpb.Order_BID
		case "ask":
			order.OrderType = rpb.Order_ASK
		}
		time.Sleep(time.Duration(delay) * time.Millisecond * 100)
		_, err := mktService.PutOrder(ctx, &rpb.PutOrderRequest{Order: order})
		if err != nil {
			grpclog.Fatalf("error putting order on row %d: %v", i, err)
		}
	}
}
