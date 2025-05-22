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
	playerQueue *queue.PlayerQueue
	logger      *log.Logger
	config      Config

	lobbyIDcnt    int64
	activeLobbies map[string]*Lobby
	lobbiesMutex  sync.RWMutex

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

	return &Matchmaker{
		playerQueue:   pq,
		logger:        lgr,
		config:        cfg,
		activeLobbies: make(map[string]*Lobby),
	}
}

func (m *Matchmaker) formLobby() {
	// Signal to the WaitGroup that a goroutine has started
	defer m.wg.Done()

	currentQueueSize := m.playerQueue.Size()
	if currentQueueSize < m.config.LobbySize {
		if currentQueueSize > 0 {
			m.logger.Printf("Matchmaker: Not enough players for a new lobby. Have: %d, Need: %d", currentQueueSize, m.config.LobbySize)
		}
		return
	}

	FinalPlayers := m.playerQueue.DeQueue(m.config.LobbySize)

	if len(FinalPlayers) == m.config.LobbySize {
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
		m.logger.Printf("Matchmaker: Lobby %s formed with %d players: %v. Queue size now: %d", lobby.ID, len(lobby.Players), playerIDs, m.playerQueue.Size())
	} else if len(FinalPlayers) > 0 { // SHOULD NOT HAPPEN (IDEALLY)
		m.logger.Printf("Matchmaker: Dequeued %d players, but needed %d. Re-queuing them (this indicates a potential issue).", len(FinalPlayers), m.config.LobbySize)
		// This part needs to be safe. If m.playerQueue.JoinQueue is not safe for concurrent calls,
		// you'll need to add a mutex around it within the PlayerQueue itself.
		// For now, assuming it's safe or will be made safe.
		for _, p := range FinalPlayers {
			m.playerQueue.JoinQueue(p) // Ensure this is goroutine-safe
		}
	}

}

func (m *Matchmaker) GetActiveLobbyCount() int {
	m.lobbiesMutex.RLock()
	defer m.lobbiesMutex.RUnlock()
	return len(m.activeLobbies)
}

func (m *Matchmaker) Start(ctx context.Context) {
	m.logger.Printf("Matchmaker started. Will attempt to form lobbies of size %d every %v.", m.config.LobbySize, m.config.Frequency)
	ticker := time.NewTicker(m.config.Frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Increment the WaitGroup counter before launching the goroutine
			m.wg.Add(1)
			go m.formLobby() // Launch formLobby in a new goroutine
		case <-ctx.Done():
			// Wait for all active formLobby goroutines to finish
			m.wg.Wait()
			m.logger.Printf("Matchmaker: Shutting down. All pending lobby formations completed.")
			return
		}
	}
}