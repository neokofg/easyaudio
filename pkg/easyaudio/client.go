package easyaudio

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type AudioClient struct {
	serverAddr string
	clientID   string
	roomID     string
	conn       *net.UDPConn
	onReceive  func([]byte)
	running    bool
}

func NewClient(serverAddr string) *AudioClient {
	return &AudioClient{
		serverAddr: serverAddr,
		clientID:   generateID(),
		running:    false,
	}
}

func (c *AudioClient) Connect(roomID string) error {
	addr, err := net.ResolveUDPAddr("udp", c.serverAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	c.conn = conn
	c.roomID = roomID
	c.running = true

	// Запускаем получение аудио
	go c.receiveAudio()

	return nil
}

func (c *AudioClient) Disconnect() error {
	c.running = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *AudioClient) SendAudio(data []byte) error {
	packet := c.createPacket(data)
	_, err := c.conn.Write(packet)
	return err
}

func (c *AudioClient) OnAudioReceived(callback func([]byte)) {
	c.onReceive = callback
}

func (c *AudioClient) receiveAudio() {
	buffer := make([]byte, MaxPacketSize)

	for c.running {
		n, err := c.conn.Read(buffer)
		if err != nil {
			continue
		}

		packet := parsePacket(buffer[:n])
		if packet == nil || packet.ClientID == c.clientID {
			continue
		}

		if c.onReceive != nil {
			c.onReceive(packet.Data)
		}
	}
}

func (c *AudioClient) createPacket(data []byte) []byte {
	roomIDBytes := []byte(c.roomID)
	clientIDBytes := []byte(c.clientID)

	// Вычисляем размер пакета
	packetSize := HeaderSize + len(roomIDBytes) + len(clientIDBytes) + len(data)
	packet := make([]byte, packetSize)

	// Записываем размеры ID
	binary.BigEndian.PutUint16(packet[0:2], uint16(len(roomIDBytes)))
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(clientIDBytes)))

	// Записываем временную метку
	binary.BigEndian.PutUint64(packet[4:12], uint64(time.Now().UnixNano()))

	// Записываем ID и данные
	copy(packet[HeaderSize:], roomIDBytes)
	copy(packet[HeaderSize+len(roomIDBytes):], clientIDBytes)
	copy(packet[HeaderSize+len(roomIDBytes)+len(clientIDBytes):], data)

	return packet
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
