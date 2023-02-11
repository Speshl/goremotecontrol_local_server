package server

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"net"
	"time"

	"github.com/Speshl/GoRemoteControl/models"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	address string
}

func NewServer(address string) *Server {
	return &Server{
		address: address,
	}
}

func (s *Server) RunServer() error {
	log.Println("Starting Controller Server...")

	errGroup, ctx := errgroup.WithContext(context.Background())
	stateChannel := s.startUDPListener(ctx, errGroup)
	latestState := s.startStateSyncer(ctx, errGroup, stateChannel)
	s.startRFWriter(ctx, errGroup, latestState)

	err := errGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) startUDPListener(ctx context.Context, errGroup *errgroup.Group) chan models.StateIface {
	returnChannel := make(chan models.StateIface, 5)
	errGroup.Go(func() error {
		ctx := context.Background()
		udpServer, err := net.ListenPacket("udp", s.address)
		if err != nil {
			return err
		}

		defer func() {
			udpServer.Close()
			close(returnChannel)
			log.Println("UDP Listener closing")
		}()

		log.Println("Listening...")
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				buffer := make([]byte, 512)
				numRead, _, err := udpServer.ReadFrom(buffer)
				if err != nil {
					log.Printf("server read error: %s\n", err.Error())
					continue
				}
				if numRead > 0 {
					var packet models.Packet
					dec := gob.NewDecoder(bytes.NewReader(buffer))
					gob.Register(models.GroundState{})
					err := dec.Decode(&packet)
					if err != nil {
						log.Printf("server decode error: %s\n", err.Error())
						continue
					}
					//log.Printf("%d bytes (Type: %s) read from %s with delay %s\n", numRead, packet.StateType, addr.String(), time.Since(packet.SentAt).String())
					returnChannel <- packet.State
				}
			}
		}
	})
	return returnChannel
}

func (s *Server) startStateSyncer(ctx context.Context, errGroup *errgroup.Group, dataChannel <-chan models.StateIface) *LatestState {
	returnMutex := LatestState{}
	errGroup.Go(func() error {
		defer log.Println("State Syncer Closing")
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case data, ok := <-dataChannel:
				if !ok {
					return nil
				}
				returnMutex.Set(data)
			}
		}
	})
	return &returnMutex
}

func (s *Server) startRFWriter(ctx context.Context, errGroup *errgroup.Group, latestState *LatestState) error {
	ticker := time.NewTicker(1000 * time.Millisecond) //RF Update rate
	errGroup.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				state, err := latestState.Get()
				if err != nil {
					log.Println("skipping rf send - latest state already used")
					continue
				}
				log.Printf("State: %+v\n", state.GetBytes())
			}
		}
	})
	return nil
}

/*
func (s *Server) processPacket(packet models.Packet) error {
	switch packet.StateType {
	case controllers.ControlSchemaGround:
		select {
			case messages <- msg
		}
		//groundState := packet.State.(controllers.GroundState)
	default:
		log.Println("failed to determing state type")
	}
	return nil
}
*/