package easyaudio

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	MaxPacketSize = 1024 * 16 // 16KB максимальный размер пакета
	HeaderSize    = 24        // Размер заголовка пакета
)

type Server struct {
	port    int
	rooms   map[string]*Room
	conn    *net.UDPConn
	mu      sync.RWMutex
	running bool
}

type Room struct {
	id      string
	clients map[string]*Client
	mu      sync.RWMutex
	conn    *net.UDPConn
}

type Client struct {
	id       string
	addr     *net.UDPAddr
	lastSeen time.Time
	room     *Room
}

type AudioPacket struct {
	RoomID    string
	ClientID  string
	Timestamp uint64
	Data      []byte
}

func NewServer(port int) *Server {
	return &Server{
		port:  port,
		rooms: make(map[string]*Room),
	}
}

func (s *Server) Start() error {
	addr := &net.UDPAddr{
		Port: s.port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	s.conn = conn
	s.running = true

	// Запускаем обработку соединений
	go s.handleConnections()
	// Запускаем очистку неактивных клиентов
	go s.cleanupInactiveClients()

	return nil
}

func (s *Server) Stop() error {
	s.running = false
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *Server) handleConnections() {
	buffer := make([]byte, MaxPacketSize)

	for s.running {
		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		// Обрабатываем пакет в отдельной горутине
		go s.handlePacket(buffer[:n], addr)
	}
}

func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	if len(data) < HeaderSize {
		return
	}

	packet := parsePacket(data)
	if packet == nil {
		return
	}

	s.mu.Lock()
	room, exists := s.rooms[packet.RoomID]
	if !exists {
		room = &Room{
			id:      packet.RoomID,
			clients: make(map[string]*Client),
		}
		s.rooms[packet.RoomID] = room
	}
	s.mu.Unlock()

	room.mu.Lock()
	client, exists := room.clients[packet.ClientID]
	if !exists {
		client = &Client{
			id:       packet.ClientID,
			addr:     addr,
			lastSeen: time.Now(),
			room:     room,
		}
		room.clients[packet.ClientID] = client
	}
	client.lastSeen = time.Now()
	room.mu.Unlock()

	// Рассылаем пакет всем клиентам в комнате
	room.broadcast(data, packet.ClientID)
}

func (r *Room) broadcast(data []byte, excludeClientID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for clientID, client := range r.clients {
		if clientID != excludeClientID {
			_, e := r.clients[clientID].room.conn.WriteToUDP(data, client.addr)
			if e != nil {
				delete(r.clients, clientID)
			}
		}
	}
}

func (s *Server) cleanupInactiveClients() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for s.running {
		<-ticker.C
		threshold := time.Now().Add(-30 * time.Second)

		s.mu.Lock()
		for _, room := range s.rooms {
			room.mu.Lock()
			for clientID, client := range room.clients {
				if client.lastSeen.Before(threshold) {
					delete(room.clients, clientID)
				}
			}
			room.mu.Unlock()
		}
		s.mu.Unlock()
	}
}

func parsePacket(data []byte) *AudioPacket {
	if len(data) < HeaderSize {
		return nil
	}

	roomIDLen := binary.BigEndian.Uint16(data[0:2])
	clientIDLen := binary.BigEndian.Uint16(data[2:4])
	timestamp := binary.BigEndian.Uint64(data[4:12])

	headerEnd := HeaderSize + int(roomIDLen) + int(clientIDLen)
	if len(data) < headerEnd {
		return nil
	}

	roomID := string(data[HeaderSize : HeaderSize+int(roomIDLen)])
	clientID := string(data[HeaderSize+int(roomIDLen) : headerEnd])
	audioData := data[headerEnd:]

	return &AudioPacket{
		RoomID:    roomID,
		ClientID:  clientID,
		Timestamp: timestamp,
		Data:      audioData,
	}
}
