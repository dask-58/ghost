package queue

import "sync"

type Player struct {
	ID string
	// Other attributes ... (Skill, Time ?)
}

type PlayerQueue struct {
	mu				sync.RWMutex // read-write mutex | multiple reader, one writer
	players 		[]Player
	playerSet 		map[string]struct{} // only to check existence O(1)
}

func __init_queue() *PlayerQueue {
	return &PlayerQueue{
		players: make([]Player, 0),
		playerSet: make(map[string]struct{}),
	}
}

func (pq *PlayerQueue) JoinQueue(player Player) bool {
	pq.mu.Lock() // full lock
	defer pq.mu.Unlock()

	// Player already in queue
	if _, exists := pq.playerSet[player.ID]; exists {
		return false
	}

	pq.players = append(pq.players, player)
	pq.playerSet[player.ID] = struct{}{}
	return true
}

func (pq *PlayerQueue) GetQueue() []Player {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	_queue := make([]Player, len(pq.players))
	copy(_queue, pq.players)
	return _queue
}

func (pq *PlayerQueue) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.players)
}

func (pq *PlayerQueue) RemovePlayers(IDS ...string) {
	// if needed after matchmaking	
}

