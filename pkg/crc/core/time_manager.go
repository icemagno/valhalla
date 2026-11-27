package core

import (
	"container/heap"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// TimeEpsilon defines the threshold for floating-point comparisons in logical time
const TimeEpsilon = 1e-9

// TimeState indicates the current time advancement state of a federate
type TimeState uint8

const (
	TimeStateGranted         TimeState = 0 // Idle, time is granted
	TimeStateAdvancingTAR     TimeState = 1 // TimeAdvanceRequest pending
	TimeStateAdvancingTARA    TimeState = 2 // TimeAdvanceRequestAvailable pending
	TimeStateAdvancingNMR     TimeState = 3 // NextMessageRequest pending
	TimeStateAdvancingNMRA    TimeState = 4 // NextMessageRequestAvailable pending
	TimeStateAdvancingFlush   TimeState = 5 // FlushQueueRequest pending
)

func (s TimeState) String() string {
	switch s {
	case TimeStateGranted:
		return "TimeGranted"
	case TimeStateAdvancingTAR:
		return "AdvancingTAR"
	case TimeStateAdvancingTARA:
		return "AdvancingTARA"
	case TimeStateAdvancingNMR:
		return "AdvancingNMR"
	case TimeStateAdvancingNMRA:
		return "AdvancingNMRA"
	case TimeStateAdvancingFlush:
		return "AdvancingFlush"
	default:
		return "Unknown"
	}
}

// TSOEventKind identifies the protocol-agnostic domain event carried by a TSOMessage.
type TSOEventKind uint8

const (
	TSOEventRaw TSOEventKind = iota
	TSOEventReflectAttributes
	TSOEventReceiveInteraction
)

// TSOMessage represents a protocol-agnostic message buffered in Time-Stamp Order (TSO).
type TSOMessage struct {
	Sender       common.FederateHandle
	Receiver     common.FederateHandle
	Timestamp    float64
	Kind         TSOEventKind
	ObjectHandle int32
	ClassHandle  int32
	Attributes   map[common.AttributeHandle][]byte
	Parameters   map[common.ParameterHandle][]byte
	Tag          []byte
	MsgType      uint8  // Legacy fallback for TSOEventRaw
	Payload      []byte // Legacy fallback for TSOEventRaw
	MerkleRoot   [32]byte
	SequenceNum  uint64
}

// tsoHeap implements a min-heap priority queue for TSOMessages ordered by Timestamp, then SequenceNum
type tsoHeap []TSOMessage

func (h tsoHeap) Len() int { return len(h) }
func (h tsoHeap) Less(i, j int) bool {
	if math.Abs(h[i].Timestamp-h[j].Timestamp) > TimeEpsilon {
		return h[i].Timestamp < h[j].Timestamp
	}
	return h[i].SequenceNum < h[j].SequenceNum
}
func (h tsoHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *tsoHeap) Push(x any) {
	*h = append(*h, x.(TSOMessage))
}
func (h *tsoHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// FederateTimeStatus encapsulates the temporal state of an individual federate
type FederateTimeStatus struct {
	Handle        common.FederateHandle
	CurrentTime   float64
	Lookahead     float64
	IsRegulating  bool
	IsConstrained bool
	State         TimeState
	TargetTime    float64
	LastHeartbeat time.Time
	Quarantined   bool
}

// TimeGrantEvent represents a granted time advance notification for a federate
type TimeGrantEvent struct {
	Federate common.FederateHandle
	Time     float64
	Messages []TSOMessage
}

const (
	// DefaultMaxTSOQueueSize caps per-federate pending TSO events to prevent OOM DoS
	DefaultMaxTSOQueueSize = 100000
)

var (
	// ErrTSOQueueFull is returned when a destination federate's TSO queue reaches capacity
	ErrTSOQueueFull = errors.New("hla time: destination TSO queue capacity exceeded")
)

// TimeManager coordinates logical time advancement, LBTS calculations, and TSO message queues
type TimeManager struct {
	mu              sync.RWMutex
	federates       map[common.FederateHandle]*FederateTimeStatus
	tsoQueues       map[common.FederateHandle]*tsoHeap
	seqCounter      uint64
	maxTSOQueueSize int
}

// NewTimeManager creates an initialized TimeManager instance
func NewTimeManager() *TimeManager {
	return &TimeManager{
		federates:       make(map[common.FederateHandle]*FederateTimeStatus),
		tsoQueues:       make(map[common.FederateHandle]*tsoHeap),
		maxTSOQueueSize: DefaultMaxTSOQueueSize,
	}
}

// SetMaxTSOQueueSize configures the maximum number of queued TSO messages per federate
func (tm *TimeManager) SetMaxTSOQueueSize(limit int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if limit > 0 {
		tm.maxTSOQueueSize = limit
	}
}

// RegisterFederate registers a federate with initial logical time 0.0 and lookahead 0.0
func (tm *TimeManager) RegisterFederate(handle common.FederateHandle) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.federates[handle]; !exists {
		tm.federates[handle] = &FederateTimeStatus{
			Handle:        handle,
			CurrentTime:   0.0,
			Lookahead:     0.0,
			IsRegulating:  false,
			IsConstrained: false,
			State:         TimeStateGranted,
			TargetTime:    0.0,
			LastHeartbeat: time.Now(),
			Quarantined:   false,
		}
		tm.tsoQueues[handle] = &tsoHeap{}
		heap.Init(tm.tsoQueues[handle])
	}
}

// UnregisterFederate removes a federate and purges its TSO queue
func (tm *TimeManager) UnregisterFederate(handle common.FederateHandle) []TimeGrantEvent {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.federates, handle)
	delete(tm.tsoQueues, handle)

	// Removing a regulating federate may unblock other advancing federates
	return tm.evaluateAdvancementsLocked()
}

// Heartbeat refreshes the vitality timestamp for a federate
func (tm *TimeManager) Heartbeat(handle common.FederateHandle) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if fed, ok := tm.federates[handle]; ok {
		fed.LastHeartbeat = time.Now()
		if fed.Quarantined {
			fed.Quarantined = false
		}
	}
}

// EnableTimeRegulation sets the federate as Time Regulating with a non-negative lookahead
func (tm *TimeManager) EnableTimeRegulation(handle common.FederateHandle, lookahead float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return common.ErrFederateNotFound
	}
	if fed.IsRegulating {
		return common.ErrTimeRegulationAlreadyEnabled
	}
	if math.IsNaN(lookahead) || math.IsInf(lookahead, 0) || lookahead < 0.0 {
		return common.ErrInvalidLookahead
	}

	fed.IsRegulating = true
	fed.Lookahead = lookahead
	fed.LastHeartbeat = time.Now()
	return nil
}

// DisableTimeRegulation removes time regulating status from the federate
func (tm *TimeManager) DisableTimeRegulation(handle common.FederateHandle) ([]TimeGrantEvent, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return nil, common.ErrFederateNotFound
	}
	if !fed.IsRegulating {
		return nil, common.ErrTimeRegulationNotEnabled
	}

	fed.IsRegulating = false
	grants := tm.evaluateAdvancementsLocked()
	return grants, nil
}

// IsConstrained reports in a thread-safe manner whether the federate is time-constrained.
func (tm *TimeManager) IsConstrained(handle common.FederateHandle) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	fed, ok := tm.federates[handle]
	return ok && fed.IsConstrained
}

// EnableTimeConstrained enables time constrained status for the federate
func (tm *TimeManager) EnableTimeConstrained(handle common.FederateHandle) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return common.ErrFederateNotFound
	}
	if fed.IsConstrained {
		return common.ErrTimeConstrainedAlreadyEnabled
	}

	fed.IsConstrained = true
	fed.LastHeartbeat = time.Now()
	return nil
}

// DisableTimeConstrained removes time constrained status and grants any pending advance
func (tm *TimeManager) DisableTimeConstrained(handle common.FederateHandle) ([]TimeGrantEvent, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return nil, common.ErrFederateNotFound
	}
	if !fed.IsConstrained {
		return nil, common.ErrTimeConstrainedNotEnabled
	}

	fed.IsConstrained = false
	grants := tm.evaluateAdvancementsLocked()
	return grants, nil
}

// ModifyLookahead changes the lookahead of a regulating federate
func (tm *TimeManager) ModifyLookahead(handle common.FederateHandle, newLookahead float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return common.ErrFederateNotFound
	}
	if !fed.IsRegulating {
		return common.ErrTimeRegulationNotEnabled
	}
	if math.IsNaN(newLookahead) || math.IsInf(newLookahead, 0) || newLookahead < 0.0 {
		return common.ErrInvalidLookahead
	}

	fed.Lookahead = newLookahead
	return nil
}

// TimeAdvanceRequest requests advancing logical time to targetTime in strict TAR mode
func (tm *TimeManager) TimeAdvanceRequest(handle common.FederateHandle, targetTime float64) ([]TimeGrantEvent, error) {
	return tm.requestAdvance(handle, targetTime, TimeStateAdvancingTAR)
}

// TimeAdvanceRequestAvailable requests advancing logical time with available bounds (TARA)
func (tm *TimeManager) TimeAdvanceRequestAvailable(handle common.FederateHandle, targetTime float64) ([]TimeGrantEvent, error) {
	return tm.requestAdvance(handle, targetTime, TimeStateAdvancingTARA)
}

// NextMessageRequest requests advancing to next message timestamp or targetTime (NMR)
func (tm *TimeManager) NextMessageRequest(handle common.FederateHandle, targetTime float64) ([]TimeGrantEvent, error) {
	return tm.requestAdvance(handle, targetTime, TimeStateAdvancingNMR)
}

// NextMessageRequestAvailable requests advancing to next message or available bounds (NMRA)
func (tm *TimeManager) NextMessageRequestAvailable(handle common.FederateHandle, targetTime float64) ([]TimeGrantEvent, error) {
	return tm.requestAdvance(handle, targetTime, TimeStateAdvancingNMRA)
}

// FlushQueueRequest requests flushing all queued messages up to targetTime
func (tm *TimeManager) FlushQueueRequest(handle common.FederateHandle, targetTime float64) ([]TimeGrantEvent, error) {
	return tm.requestAdvance(handle, targetTime, TimeStateAdvancingFlush)
}

// requestAdvance registers an advance request and triggers resolution
func (tm *TimeManager) requestAdvance(handle common.FederateHandle, targetTime float64, state TimeState) ([]TimeGrantEvent, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return nil, common.ErrFederateNotFound
	}
	if fed.State != TimeStateGranted {
		return nil, common.ErrTimeAdvanceAlreadyInProgress
	}
	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return nil, common.ErrInvalidLogicalTime
	}
	if targetTime < fed.CurrentTime-TimeEpsilon {
		return nil, common.ErrLogicalTimeAlreadyPassed
	}

	fed.State = state
	fed.TargetTime = targetTime
	fed.LastHeartbeat = time.Now()

	// If the federate is not constrained, immediately grant the requested advance
	if !fed.IsConstrained {
		fed.CurrentTime = targetTime
		fed.State = TimeStateGranted
		var msgs []TSOMessage
		if q, hasQ := tm.tsoQueues[handle]; hasQ && q.Len() > 0 {
			msgs = tm.drainQueueUpToLocked(q, targetTime)
		}
		grants := []TimeGrantEvent{
			{
				Federate: handle,
				Time:     targetTime,
				Messages: msgs,
			},
		}
		// Also evaluate if this advance unblocks other federates
		furtherGrants := tm.evaluateAdvancementsLocked()
		return append(grants, furtherGrants...), nil
	}

	return tm.evaluateAdvancementsLocked(), nil
}

// EnqueueTSOMessage places a message into the destination federate's TSO priority queue
func (tm *TimeManager) EnqueueTSOMessage(msg TSOMessage) ([]TimeGrantEvent, error) {
	if math.IsNaN(msg.Timestamp) || math.IsInf(msg.Timestamp, 0) {
		return nil, common.ErrInvalidLogicalTime
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()

	sender, ok := tm.federates[msg.Sender]
	if ok && sender.IsRegulating {
		minTime := sender.CurrentTime + sender.Lookahead - TimeEpsilon
		if msg.Timestamp < minTime {
			return nil, common.ErrInvalidLogicalTime
		}
	}

	q, ok := tm.tsoQueues[msg.Receiver]
	if !ok {
		return nil, common.ErrFederateNotFound
	}
	if tm.maxTSOQueueSize > 0 && q.Len() >= tm.maxTSOQueueSize {
		return nil, ErrTSOQueueFull
	}

	tm.seqCounter++
	msg.SequenceNum = tm.seqCounter
	heap.Push(q, msg)

	// A new message may allow NMR/NMRA advancing federates to complete
	grants := tm.evaluateAdvancementsLocked()
	return grants, nil
}

// CalculateLBTS calculates the Lower Bound on Time Stamp for a specific federate
func (tm *TimeManager) CalculateLBTS(handle common.FederateHandle) float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.calculateLBTSLocked(handle)
}

// calculateLBTSLocked internal calculation without locking
func (tm *TimeManager) calculateLBTSLocked(handle common.FederateHandle) float64 {
	minBound := math.MaxFloat64

	// Minimum impending time of all regulating peers (excluding quarantined)
	for h, fed := range tm.federates {
		if h == handle || !fed.IsRegulating || fed.Quarantined {
			continue
		}

		var impending float64
		if fed.State == TimeStateGranted {
			impending = fed.CurrentTime + fed.Lookahead
		} else {
			impending = fed.TargetTime + fed.Lookahead
		}

		if impending < minBound {
			minBound = impending
		}
	}

	return minBound
}

// evaluateAdvancementsLocked iterates over advancing federates in deterministic handle order
// and produces TimeGrantEvents. Uses an O(N) two-minimum LBTS precomputation per pass so
// each federate's LBTS lookup is O(1) instead of O(N).
func (tm *TimeManager) evaluateAdvancementsLocked() []TimeGrantEvent {
	var events []TimeGrantEvent

	handles := make([]common.FederateHandle, 0, len(tm.federates))
	for h := range tm.federates {
		handles = append(handles, h)
	}
	sort.Slice(handles, func(i, j int) bool { return handles[i] < handles[j] })

	for {
		progress := false

		// Precompute the two smallest impending times across all regulating, non-quarantined federates in O(N).
		min1 := math.MaxFloat64
		var min1Handle common.FederateHandle
		min2 := math.MaxFloat64
		for _, h := range handles {
			f := tm.federates[h]
			if f == nil || !f.IsRegulating || f.Quarantined {
				continue
			}
			impending := f.TargetTime + f.Lookahead
			if f.State == TimeStateGranted {
				impending = f.CurrentTime + f.Lookahead
			}
			if impending < min1 {
				min2 = min1
				min1 = impending
				min1Handle = h
			} else if impending < min2 {
				min2 = impending
			}
		}

		updateMinOnGrant := func(h common.FederateHandle, f *FederateTimeStatus) {
			if !f.IsRegulating || f.Quarantined {
				return
			}
			impending := f.CurrentTime + f.Lookahead
			if h == min1Handle {
				min1 = impending
			} else if impending < min1 {
				min2 = min1
				min1 = impending
				min1Handle = h
			} else if impending < min2 {
				min2 = impending
			}
		}

		for _, handle := range handles {
			fed := tm.federates[handle]
			if fed == nil || fed.State == TimeStateGranted {
				continue
			}

			lbts := min1
			if handle == min1Handle {
				lbts = min2
			}
			q := tm.tsoQueues[handle]

			switch fed.State {
			case TimeStateAdvancingTAR:
				// TAR: Grant target time only when targetTime <= LBTS
				if fed.TargetTime <= lbts+TimeEpsilon {
					msgs := tm.drainQueueUpToLocked(q, fed.TargetTime)
					fed.CurrentTime = fed.TargetTime
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     fed.TargetTime,
						Messages: msgs,
					})
					progress = true
				}

			case TimeStateAdvancingTARA:
				// TARA: Grant target if target <= LBTS; otherwise grant LBTS if > CurrentTime
				if fed.TargetTime <= lbts+TimeEpsilon {
					msgs := tm.drainQueueUpToLocked(q, fed.TargetTime)
					fed.CurrentTime = fed.TargetTime
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     fed.TargetTime,
						Messages: msgs,
					})
					progress = true
				} else if lbts > fed.CurrentTime+TimeEpsilon && lbts < math.MaxFloat64 {
					msgs := tm.drainQueueUpToLocked(q, lbts)
					fed.CurrentTime = lbts
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     lbts,
						Messages: msgs,
					})
					progress = true
				}

			case TimeStateAdvancingNMR:
				// NMR: Grant earliest queued message timestamp if <= TargetTime and <= LBTS
				if q.Len() > 0 {
					earliest := (*q)[0].Timestamp
					if earliest <= fed.TargetTime+TimeEpsilon && earliest <= lbts+TimeEpsilon {
						msgs := tm.drainQueueUpToLocked(q, earliest)
						fed.CurrentTime = earliest
						fed.State = TimeStateGranted
						fed.LastHeartbeat = time.Now()
						updateMinOnGrant(handle, fed)
						events = append(events, TimeGrantEvent{
							Federate: handle,
							Time:     earliest,
							Messages: msgs,
						})
						progress = true
						continue
					}
				}
				// If no qualifying message, grant TargetTime if <= LBTS
				if fed.TargetTime <= lbts+TimeEpsilon {
					msgs := tm.drainQueueUpToLocked(q, fed.TargetTime)
					fed.CurrentTime = fed.TargetTime
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     fed.TargetTime,
						Messages: msgs,
					})
					progress = true
				}

			case TimeStateAdvancingNMRA:
				// NMRA: Like NMR, but falls back to granting LBTS if available
				if q.Len() > 0 {
					earliest := (*q)[0].Timestamp
					if earliest <= fed.TargetTime+TimeEpsilon && earliest <= lbts+TimeEpsilon {
						msgs := tm.drainQueueUpToLocked(q, earliest)
						fed.CurrentTime = earliest
						fed.State = TimeStateGranted
						fed.LastHeartbeat = time.Now()
						updateMinOnGrant(handle, fed)
						events = append(events, TimeGrantEvent{
							Federate: handle,
							Time:     earliest,
							Messages: msgs,
						})
						progress = true
						continue
					}
				}
				if fed.TargetTime <= lbts+TimeEpsilon {
					msgs := tm.drainQueueUpToLocked(q, fed.TargetTime)
					fed.CurrentTime = fed.TargetTime
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     fed.TargetTime,
						Messages: msgs,
					})
					progress = true
				} else if lbts > fed.CurrentTime+TimeEpsilon && lbts < math.MaxFloat64 {
					msgs := tm.drainQueueUpToLocked(q, lbts)
					fed.CurrentTime = lbts
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     lbts,
						Messages: msgs,
					})
					progress = true
				}

			case TimeStateAdvancingFlush:
				// Flush: Deliver all messages up to TargetTime or LBTS
				effective := math.Min(fed.TargetTime, lbts)
				if effective >= fed.CurrentTime-TimeEpsilon {
					msgs := tm.drainQueueUpToLocked(q, effective)
					fed.CurrentTime = effective
					fed.State = TimeStateGranted
					fed.LastHeartbeat = time.Now()
					updateMinOnGrant(handle, fed)
					events = append(events, TimeGrantEvent{
						Federate: handle,
						Time:     effective,
						Messages: msgs,
					})
					progress = true
				}
			}
		}

		if !progress {
			break
		}
	}

	return events
}

// drainQueueUpToLocked removes and returns all messages from heap with timestamp <= upTo
func (tm *TimeManager) drainQueueUpToLocked(q *tsoHeap, upTo float64) []TSOMessage {
	var msgs []TSOMessage
	for q.Len() > 0 {
		top := (*q)[0]
		if top.Timestamp <= upTo+TimeEpsilon {
			heap.Pop(q)
			msgs = append(msgs, top)
		} else {
			break
		}
	}
	return msgs
}

// EvictStalledFederates detects regulating federates that have stopped sending heartbeats
// within timeout and quarantines them from LBTS calculations to prevent deadlock.
// Federates that are actively blocked waiting for an advance grant (State != TimeStateGranted)
// are waiting for peer progression and are not stalled.
func (tm *TimeManager) EvictStalledFederates(timeout time.Duration) ([]common.FederateHandle, []TimeGrantEvent) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	var evicted []common.FederateHandle
	for handle, fed := range tm.federates {
		if fed.IsRegulating && !fed.Quarantined {
			if fed.State != TimeStateGranted {
				continue
			}
			if now.Sub(fed.LastHeartbeat) > timeout {
				fed.Quarantined = true
				evicted = append(evicted, handle)
			}
		}
	}

	var grants []TimeGrantEvent
	if len(evicted) > 0 {
		// Quarantining regulating nodes lowers LBTS and unblocks waiting nodes
		grants = tm.evaluateAdvancementsLocked()
	}

	return evicted, grants
}

// GetStatus returns a snapshot of a federate's time status
func (tm *TimeManager) GetStatus(handle common.FederateHandle) (FederateTimeStatus, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	fed, ok := tm.federates[handle]
	if !ok {
		return FederateTimeStatus{}, common.ErrFederateNotFound
	}
	return *fed, nil
}
