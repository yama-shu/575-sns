// Package circuitbreaker は、応答しなくなった下流サービスへの呼び出しを止める。
//
// 死んでいると分かっているサービスを呼び続けると、両側で状況が悪化する。
//
//   - 呼ぶ側: タイムアウトを待つ分だけリソースを消費する
//   - 呼ばれる側: 再起動して回復しようとしているところに負荷がかかる
//
// 状態遷移は詳細設計 03 §4 に従う。
//
//	Closed   --直近 N 回中 M 回以上失敗--> Open
//	Open     --一定時間の経過-----------> HalfOpen
//	HalfOpen --試行が成功---------------> Closed
//	HalfOpen --試行が失敗---------------> Open
//
// 状態はプロセス内のメモリに持つ。インスタンス間で共有しないのは、
// 守る対象が「自インスタンスの資源」であり、
// 自分が失敗したかどうかは自分だけが知っていればよいためである。
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State は遮断器の状態。
type State int

const (
	// StateClosed は通常状態。下流を呼ぶ。
	StateClosed State = iota
	// StateOpen は開放状態。下流を呼ばずに即座に失敗させる。
	StateOpen
	// StateHalfOpen は半開状態。1件だけ試しに呼び、回復したかを確認する。
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	}
	return "unknown"
}

// ErrOpen は開放中に呼び出されたことを表す。
//
// **下流を呼んだ結果ではない。** 呼ばずに返している。
var ErrOpen = errors.New("サーキットブレーカーが開放中です")

// Settings は遮断器の設定。詳細設計 03 §4 の値を既定とする。
type Settings struct {
	// WindowSize は失敗率を数える直近の呼び出し回数。
	WindowSize int
	// FailureThreshold は窓の中で開放に至る失敗回数。
	FailureThreshold int
	// OpenDuration は開放してから半開へ移るまでの時間。
	OpenDuration time.Duration
	// Now は現在時刻。テストで時刻を進めるために差し替える。
	Now func() time.Time
	// OnStateChange は状態が変わったときに呼ばれる。
	//
	// 遮断器は「呼ばない」ことで働くため、外から見ると
	// **何も起きていないのと区別がつかない。** 遷移を記録して観測できるようにする。
	// ロックを保持したまま呼ぶため、この中から遮断器を操作してはならない。
	OnStateChange func(from, to State)
}

// DefaultSettings は詳細設計 03 §4 の値。
func DefaultSettings() Settings {
	return Settings{
		WindowSize:       20,
		FailureThreshold: 10,
		OpenDuration:     30 * time.Second,
		Now:              time.Now,
	}
}

// Breaker は1つの下流サービスに対する遮断器。複数のゴルーチンから使える。
type Breaker struct {
	settings Settings

	mu    sync.Mutex
	state State
	// window は直近の結果。true が失敗。長さは WindowSize を超えない。
	window []bool
	// failures は window に含まれる失敗の数。数え直しを避けるために持つ。
	failures int
	// openedAt は開放した時刻。半開へ移る判断に使う。
	openedAt time.Time
	// halfOpenInFlight は半開の試行が進行中か。
	//
	// **半開で通すのは1件だけ。** 同時に複数を通すと、
	// 回復途中の下流に一斉に負荷をかけることになる。
	halfOpenInFlight bool
}

// New は遮断器をつくる。設定の未指定の項目は既定値で埋める。
func New(s Settings) *Breaker {
	d := DefaultSettings()
	if s.WindowSize <= 0 {
		s.WindowSize = d.WindowSize
	}
	if s.FailureThreshold <= 0 {
		s.FailureThreshold = d.FailureThreshold
	}
	if s.OpenDuration <= 0 {
		s.OpenDuration = d.OpenDuration
	}
	if s.Now == nil {
		s.Now = d.Now
	}
	return &Breaker{
		settings: s,
		state:    StateClosed,
		window:   make([]bool, 0, s.WindowSize),
	}
}

// Do は fn を実行する。開放中は fn を呼ばずに ErrOpen を返す。
func (b *Breaker) Do(fn func() error) error {
	if err := b.before(); err != nil {
		return err
	}
	err := fn()
	b.after(err == nil)
	return err
}

// State は現在の状態を返す。開放時間が経過していれば半開へ移す。
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.promoteToHalfOpen()
	return b.state
}

// before は呼び出しの可否を判断する。
func (b *Breaker) before() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.promoteToHalfOpen()

	switch b.state {
	case StateOpen:
		return ErrOpen
	case StateHalfOpen:
		// 試行が進行中なら通さない。1件で回復を判断する。
		if b.halfOpenInFlight {
			return ErrOpen
		}
		b.halfOpenInFlight = true
	case StateClosed:
	}
	return nil
}

// after は結果を記録し、必要なら状態を変える。
func (b *Breaker) after(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == StateHalfOpen {
		b.halfOpenInFlight = false
		if success {
			b.close()
		} else {
			b.open()
		}
		return
	}

	b.record(!success)
	if b.failures >= b.settings.FailureThreshold {
		b.open()
	}
}

// promoteToHalfOpen は開放時間が過ぎていれば半開へ移す。呼び出し元でロックを取る。
func (b *Breaker) promoteToHalfOpen() {
	if b.state != StateOpen {
		return
	}
	if b.settings.Now().Sub(b.openedAt) < b.settings.OpenDuration {
		return
	}
	b.transition(StateHalfOpen)
	b.halfOpenInFlight = false
}

// record は結果を窓に加える。呼び出し元でロックを取る。
func (b *Breaker) record(failed bool) {
	if len(b.window) == b.settings.WindowSize {
		if b.window[0] {
			b.failures--
		}
		// 前へ詰める。b.window[1:] で済ませると先頭が進み続け、
		// append のたびに確保し直すことになる。
		copy(b.window, b.window[1:])
		b.window = b.window[:len(b.window)-1]
	}
	b.window = append(b.window, failed)
	if failed {
		b.failures++
	}
}

// open は開放する。呼び出し元でロックを取る。
func (b *Breaker) open() {
	b.transition(StateOpen)
	b.openedAt = b.settings.Now()
	b.halfOpenInFlight = false
	// 窓を捨てる。捨てないと、半開から閉じた直後に
	// 開放前の失敗が残っていて即座に再開放する。
	b.reset()
}

// close は閉じる。呼び出し元でロックを取る。
func (b *Breaker) close() {
	b.transition(StateClosed)
	b.reset()
}

// transition は状態を変え、変化があれば通知する。呼び出し元でロックを取る。
func (b *Breaker) transition(to State) {
	from := b.state
	b.state = to
	if from != to && b.settings.OnStateChange != nil {
		b.settings.OnStateChange(from, to)
	}
}

func (b *Breaker) reset() {
	b.window = b.window[:0]
	b.failures = 0
}
