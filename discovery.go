package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"

	"github.com/dgraph-io/badger"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

type databaseconfig struct {
	dbname string
	nNodes int
	nodes  []string
	host   host.Host
	db     *badger.DB
}

var dbconfig databaseconfig

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	str, _ := rw.ReadString('\n')
	fmt.Printf("\x1b[32m%s\x1b[0m", str)

	if str == "connect\n" {
		connectNode(rw)
	} else if str == "add nodes\n" {
		addNodes(rw)
	} else if str == "start database\n" {
		startDatabase()
	} else if str == "set\n" {
		p2pSetValue(rw)
	} else if str == "get\n" {
		p2pGetValue(rw)
	}
}

func connectNode(rw *bufio.ReadWriter) {
	if len(dbconfig.nodes) == dbconfig.nNodes-1 {
		_, _ = rw.WriteString(fmt.Sprintf("%s\n", "decline"))
		_ = rw.Flush()
	} else {
		_, _ = rw.WriteString(fmt.Sprintf("%s\n", dbconfig.dbname))
		_, _ = rw.WriteString(fmt.Sprintf("%d\n", dbconfig.nNodes))
		_ = rw.Flush()

		str, _ := rw.ReadString('\n')
		fmt.Printf("\x1b[32m%s\x1b[0m", str)
		dbconfig.nodes = append(dbconfig.nodes, str)

		_, _ = rw.WriteString(fmt.Sprintf("%d\n", dbconfig.nNodes-1))

		for _, nodeID := range dbconfig.nodes {
			if nodeID == str {
				continue
			} else {
				_, _ = rw.WriteString(fmt.Sprintf("%s\n", nodeID))
			}
		}

		_ = rw.Flush()

		for _, nodeID := range dbconfig.nodes {
			if nodeID == str {
				continue
			} else {
				stream, _ := dbconfig.host.NewStream(context.Background(), peer.ID(nodeID), protocol.ID("/picolo/1.0"))
				rw2 := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
				_, _ = rw2.WriteString(fmt.Sprintf("%s\n", "add nodes"))
				_, _ = rw2.WriteString(fmt.Sprintf("%d\n", 1))
				_, _ = rw2.WriteString(fmt.Sprintf("%s\n", nodeID))
				_ = rw2.Flush()
			}
		}

		if len(dbconfig.nodes) == dbconfig.nNodes-1 {
			for _, nodeID := range dbconfig.nodes {
				stream, _ := dbconfig.host.NewStream(context.Background(), peer.ID(nodeID), protocol.ID("/picolo/1.0"))
				rw2 := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
				_, _ = rw2.WriteString(fmt.Sprintf("%s\n", "start database"))
				_ = rw2.Flush()
			}
			startDatabase()
		}
	}
}

func addNodes(rw *bufio.ReadWriter) {
	str, _ := rw.ReadString('\n')
	fmt.Printf("\x1b[32m%s\x1b[0m", str)
	n, _ := strconv.Atoi(str)
	for i := 0; i < n; i++ {
		str, _ = rw.ReadString('\n')
		fmt.Printf("\x1b[32m%s\x1b[0m", str)
		dbconfig.nodes = append(dbconfig.nodes, str)
	}
}

func startDatabase() {
	for _, nodeID := range dbconfig.nodes {
		fmt.Printf("\x1b[32m%s\x1b[0m\n", nodeID)
	}
	opts := badger.DefaultOptions
	opts.Dir = "/tmp/badger"
	opts.ValueDir = "/tmp/badger"
	dbconfig.db, _ = badger.Open(opts)

	http.HandleFunc("/get", httpGetValueHandler)
	http.HandleFunc("/set", httpSetValueHandler)
	http.ListenAndServe(":8080", nil)
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}

func announceDatabase(routingDiscovery *discovery.RoutingDiscovery) {
	fmt.Println("Announcing ourselves...")
	discovery.Advertise(context.Background(), routingDiscovery, "Database Bootstrap")
	fmt.Println("Successfully announced!")
}

func connectToDatabase(routingDiscovery *discovery.RoutingDiscovery) {
	fmt.Println("Searching for other peers...")
	for {
		peerChan, err := routingDiscovery.FindPeers(context.Background(), "Database Bootstrap")
		if err != nil {
			panic(err)
		}

		for peer := range peerChan {
			if peer.ID == dbconfig.host.ID() {
				continue
			}
			fmt.Println("Found peer:", peer)

			fmt.Println("Connecting to:", peer)
			stream, err := dbconfig.host.NewStream(context.Background(), peer.ID, protocol.ID("/picolo/1.0"))

			if err != nil {
				fmt.Println("Connection failed:", err)
				continue
			} else {
				rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

				_, _ = rw.WriteString(fmt.Sprintf("%s\n", "connect"))
				_ = rw.Flush()

				str, _ := rw.ReadString('\n')
				fmt.Printf("\x1b[32m%s\x1b[0m", str)
				if str == "decline\n" {
					break
				}
				dbconfig.dbname = str

				str, _ = rw.ReadString('\n')
				fmt.Printf("\x1b[32m%s\x1b[0m", str)
				dbconfig.nNodes, _ = strconv.Atoi(str)

				dbconfig.nodes = append(dbconfig.nodes, string(peer.ID))

				_, _ = rw.WriteString(fmt.Sprintf("%s\n", dbconfig.host.ID()))
				_ = rw.Flush()

				addNodes(rw)

				fmt.Println("Connected to:", peer)
				break
			}
		}
	}
}

func main() {
	sourcePort := flag.Int("p", 3001, "Port number")
	mode := flag.String("m", "announce", "Mode")
	dbname := flag.String("d", "mydb", "Database Name")
	nNodes := flag.Int("n", 1, "Number of nodes")

	flag.Parse()

	dbconfig.dbname = *dbname
	dbconfig.nNodes = *nNodes

	ctx := context.Background()
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *sourcePort)),
	)

	dbconfig.host = host
	if err != nil {
		panic(err)
	}

	dbconfig.host.SetStreamHandler(protocol.ID("/picolo/1.0"), handleStream)
	kademliaDHT, err := dht.New(ctx, dbconfig.host)
	if err != nil {
		panic(err)
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := dbconfig.host.Connect(ctx, *peerinfo); err != nil {
			} else {
				fmt.Println("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)

	if *mode == "connect" {
		connectToDatabase(routingDiscovery)
	} else {
		announceDatabase(routingDiscovery)
	}

	select {}
}
