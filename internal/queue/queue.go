package queue

type Player struct {
	ID string
}

// slice to store players in memory.
// needs synchronization for concurrency.
var queue []Player

func isinQueue(id string) bool {
	for _, p := range queue {
		if p.ID == id {
			return true
		}
	}
	return false
}

func JoinQueue(player Player) bool {
	if !isinQueue(player.ID) {
		queue = append(queue, player)
		return true
	}
	return false
}

func GetQueue() []Player {
	return queue
}