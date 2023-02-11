package client

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/0xcafed00d/joystick"
	"github.com/Speshl/GoRemoteControl/client/controllers"
	"github.com/Speshl/GoRemoteControl/models"
)

type Client struct {
	address string
	cfgPath string
}

func NewClient(address string, cfgPath string) *Client {
	client := Client{
		address: address,
		cfgPath: cfgPath,
	}
	return &client
}

func (c *Client) RunClient() error {
	udpServer, err := net.ResolveUDPAddr("udp", ":1053")
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		return err
	}
	defer conn.Close()

	joySticks, err := c.getJoysticks()
	if err != nil {
		return err
	}
	defer func() {
		for _, js := range joySticks {
			js.Close()
		}
	}()

	controller, err := controllers.CreateController(joySticks, c.cfgPath)
	if err != nil {
		return err
	}
	log.Println("Start Sending...")
	for {
		state, err := controller.GetUpdatedState()
		if err != nil {
			return fmt.Errorf("failed getting controller state- %w", err)
		}

		statePacket := models.Packet{
			StateType: state.GetType(),
			State:     state,
			SentAt:    time.Now(),
		}
		var buffer bytes.Buffer
		encoder := gob.NewEncoder(&buffer)
		gob.Register(models.GroundState{})
		err = encoder.Encode(statePacket)
		if err != nil {
			return err
		}
		_, err = conn.Write(buffer.Bytes())
		if err != nil {
			return err
		}
		//log.Printf("%+v\n", state)
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *Client) getJoysticks() ([]joystick.Joystick, error) {
	joySticks := make([]joystick.Joystick, 0)

	for i := 0; i < 10; i++ {
		js, err := joystick.Open(i)
		if err != nil {
			if i == 0 {
				return nil, fmt.Errorf("no joysticks found - %w\n", err)
			}
			break //not an issue if we got atleast 1
		}
		log.Printf("Joystick Name: %s", js.Name())
		log.Printf("   Axis Count: %d", js.AxisCount())
		log.Printf(" Button Count: %d", js.ButtonCount())
		joySticks = append(joySticks, js)
	}
	return joySticks, nil
}