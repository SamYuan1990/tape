package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Peer struct {
	BlkSize, txCnt uint64
	TxC            chan struct{}
}

func (p *Peer) ProcessProposal(context.Context, *peer.SignedProposal) (*peer.ProposalResponse, error) {
	return &peer.ProposalResponse{Response: &peer.Response{Status: 200}}, nil
}

func (p *Peer) Deliver(peer.Deliver_DeliverServer) error {
	panic("Not implemented")
	return nil
}

func (p *Peer) DeliverFiltered(srv peer.Deliver_DeliverFilteredServer) error {
	_, err := srv.Recv()
	if err != nil {
		panic("expect no recv error")
	}
	srv.Send(&peer.DeliverResponse{})

	for range p.TxC {
		p.txCnt++
		if p.txCnt%p.BlkSize == 0 {
			srv.Send(&peer.DeliverResponse{Type: &peer.DeliverResponse_FilteredBlock{
				FilteredBlock: &peer.FilteredBlock{FilteredTransactions: make([]*peer.FilteredTransaction, p.BlkSize)}}})
		}
	}

	return nil
}

func (n *Peer) DeliverWithPrivateData(peer.Deliver_DeliverWithPrivateDataServer) error {
	panic("Not implemented")
	return nil
}

type Orderer struct {
	cnt uint64
	TxC chan struct{}
}

func (o *Orderer) Deliver(orderer.AtomicBroadcast_DeliverServer) error {
	panic("Not implemented")
	return nil
}

func (o *Orderer) Broadcast(srv orderer.AtomicBroadcast_BroadcastServer) error {
	for {
		_, err := srv.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			fmt.Println(err)
			return err
		}

		o.TxC <- struct{}{}

		err = srv.Send(&orderer.BroadcastResponse{Status: common.Status_SUCCESS})
		if err != nil {
			return err
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", "127.0.0.1:10086")
	if err != nil {
		panic(err)
	}

	mtls := false

	if len(os.Args) > 1 {
		mtls, err = strconv.ParseBool(os.Args[1])
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Start listening on localhost...")

	blockC := make(chan struct{}, 1000)

	p := &Peer{
		BlkSize: 10,
		TxC:     blockC,
	}

	o := &Orderer{
		TxC: blockC,
	}
	grpcServer := grpc.NewServer()

	if mtls {
		fmt.Println("Enable mtls")
		peerCert, err := tls.LoadX509KeyPair("organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt",
			"organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.key")
		if err != nil {
			fmt.Println("load peer cert/key error:", err)
			return
		}
		caCert, err := ioutil.ReadFile("organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt")
		if err != nil {
			fmt.Println("read ca cert file error:", err)
			return
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		ta := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{peerCert},
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		})
		grpcServer = grpc.NewServer(grpc.Creds(ta))
	}
	peer.RegisterEndorserServer(grpcServer, p)
	peer.RegisterDeliverServer(grpcServer, p)
	orderer.RegisterAtomicBroadcastServer(grpcServer, o)

	err = grpcServer.Serve(lis)
	if err != nil {
		panic(err)
	}
}
