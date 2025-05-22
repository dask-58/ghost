package matchmaker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dask-58/ghost/internal/queue"
)

const (
	DefaultLobbySize = 100
	DefaultFrequency = 3 * time.Second
)

type Lobby struct {
	ID        string
	Players   []queue.Player
	CreatedAt time.Time
	// server_id ? status ?
}

type Config struct {
	LobbySize int
	Frequency time.Duration
}

type Matchmaker struct {
	playerQueue 	*queue.PlayerQueue
	logger      	*log.Logger
	config      	Config
	minLobbySize 	int64

	lobbyIDcnt    	int64
	activeLobbies 	map[string]*Lobby
	lobbiesMutex  	sync.RWMutex

	// Add a WaitGroup to track active goroutines
	// This is important for graceful shutdown
	wg sync.WaitGroup
}

func NewMatchmaker(pq *queue.PlayerQueue, lgr *log.Logger, cfg Config) *Matchmaker {
	if cfg.LobbySize <= 0 {
		cfg.LobbySize = DefaultLobbySize
	}
	if cfg.Frequency <= 0 {
		cfg.Frequency = DefaultFrequency
	}

	m := &Matchmaker{
		playerQueue:   pq,
		logger:        lgr,
		config:        cfg,
		activeLobbies: make(map[string]*Lobby),
	}

	m.minLobbySize = int64(float64(cfg.LobbySize) * 0.75)
	return m
}

func (m *Matchmaker) formLobby() {
	// Signal to the WaitGroup that a goroutine has started
	defer m.wg.Done()

	currentQueueSize := m.playerQueue.Size()
	if currentQueueSize < int(m.minLobbySize) {
		if currentQueueSize > 0 {
			m.logger.Printf("Not enough players for a new lobby. Have: %d, Need at least: %d", currentQueueSize, m.minLobbySize)
		}
		return
	}

	// Dequeue up to LobbySize players, but at least minLobbySize
	FinalPlayers := m.playerQueue.DeQueue(m.config.LobbySize)
	if len(FinalPlayers) >= int(m.minLobbySize) {
		newID := atomic.AddInt64(&m.lobbyIDcnt, 1)
		lobby := Lobby{
			ID:        fmt.Sprintf("lobby-%d", newID),
			Players:   FinalPlayers,
			CreatedAt: time.Now(),
		}

		m.lobbiesMutex.Lock()
		m.activeLobbies[lobby.ID] = &lobby
		m.lobbiesMutex.Unlock()

		playerIDs := make([]string, len(lobby.Players))
		for i, p := range lobby.Players {
			playerIDs[i] = p.ID
		}
		m.logger.Printf("Lobby %s formed with %d players: %v. Queue size now: %d", lobby.ID, len(lobby.Players), playerIDs, m.playerQueue.Size())
	} else {
		m.logger.Printf("Dequeued %d players, but needed at least %d. Not forming lobby.", len(FinalPlayers), m.minLobbySize)
	}
}

func (m *Matchmaker) GetActiveLobbyCount() int {
	m.lobbiesMutex.RLock()
	defer m.lobbiesMutex.RUnlock()
	return len(m.activeLobbies)
}

func (m *Matchmaker) Start(ctx context.Context) {
	m.logger.Printf("Matchmaker started. Will attempt to form lobbies every %v.", m.config.Frequency)
	ticker := time.NewTicker(m.config.Frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.formLobby() // Run synchronously
		case <-ctx.Done():
			m.logger.Println("Matchmaker: Shutting down.")
			return
		}
	}
}