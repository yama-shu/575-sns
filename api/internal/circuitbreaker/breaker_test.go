package circuitbreaker_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yama-shu/575-sns/api/internal/circuitbreaker"
)

var errFail = errors.New("失敗")

// fakeClock は時刻を手で進める時計。
//
// **実時間を待たない。** 開放時間 30 秒をテストで実際に待つと、
// テストが遅くなるだけでなく、実行環境の負荷で不安定になる。
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// settings はテスト用の設定。既定値と同じ比率を保ったまま小さくする。
func settings(clock *fakeClock) circuitbreaker.Settings {
	return circuitbreaker.Settings{
		WindowSize:       20,
		FailureThreshold: 10,
		OpenDuration:     30 * time.Second,
		Now:              clock.Now,
	}
}

// fail は失敗する呼び出しを n 回行い、実際に fn が呼ばれた回数を返す。
func fail(b *circuitbreaker.Breaker, n int) int {
	calls := 0
	for range n {
		_ = b.Do(func() error {
			calls++
			return errFail
		})
	}
	return calls
}

func TestClosedByDefault(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("初期状態が closed でない: %v", b.State())
	}
}

// 詳細設計 03 §4: 直近 20 回中 10 回以上失敗で開放する。
func TestOpensAtThreshold(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))

	// 9回目までは閉じたまま。境界の手前を固定する。
	fail(b, 9)
	if b.State() != circuitbreaker.StateClosed {
		t.Fatalf("9回の失敗で開放された: %v", b.State())
	}

	fail(b, 1)
	if b.State() != circuitbreaker.StateOpen {
		t.Errorf("10回の失敗で開放されない: %v", b.State())
	}
}

// 成功が混ざれば、窓の中の失敗が閾値に届かない限り開かない。
func TestDoesNotOpenWhenSuccessesDominate(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))

	// 失敗9・成功11 = 直近20回中9失敗。閾値に1足りない。
	for range 9 {
		_ = b.Do(func() error { return errFail })
	}
	for range 11 {
		_ = b.Do(func() error { return nil })
	}
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("閾値未満で開放された: %v", b.State())
	}
}

// 古い失敗は窓から外れる。外れないと、いつまでも過去の失敗で開放する。
func TestOldFailuresLeaveTheWindow(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))

	fail(b, 9)
	// 成功を 20 回。窓（20）が成功で埋まり、失敗はすべて押し出される。
	for range 20 {
		_ = b.Do(func() error { return nil })
	}
	// ここから 9 回失敗しても、押し出されていれば閾値に届かない。
	fail(b, 9)
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("窓から外れた失敗が数えられている: %v", b.State())
	}
}

// 開放中は下流を呼ばない。**これが遮断器の目的そのものである。**
func TestOpenDoesNotCallDownstream(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))
	fail(b, 10)

	calls := fail(b, 5)
	if calls != 0 {
		t.Errorf("開放中に %d 回呼ばれた", calls)
	}
}

func TestOpenReturnsErrOpen(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))
	fail(b, 10)

	err := b.Do(func() error { return nil })
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Errorf("ErrOpen を期待したが %v", err)
	}
}

// 開放から一定時間で半開へ移る。
func TestHalfOpenAfterOpenDuration(t *testing.T) {
	clock := newClock()
	b := circuitbreaker.New(settings(clock))
	fail(b, 10)

	// 経過前は開いたまま。境界の手前を固定する。
	clock.advance(29 * time.Second)
	if b.State() != circuitbreaker.StateOpen {
		t.Fatalf("30秒前に半開へ移った: %v", b.State())
	}

	clock.advance(1 * time.Second)
	if b.State() != circuitbreaker.StateHalfOpen {
		t.Errorf("30秒経過しても半開にならない: %v", b.State())
	}
}

// 半開の試行が成功すれば閉じる。
func TestHalfOpenClosesOnSuccess(t *testing.T) {
	clock := newClock()
	b := circuitbreaker.New(settings(clock))
	fail(b, 10)
	clock.advance(30 * time.Second)

	calls := 0
	if err := b.Do(func() error { calls++; return nil }); err != nil {
		t.Fatalf("半開で試行できない: %v", err)
	}
	if calls != 1 {
		t.Errorf("半開で下流が呼ばれていない: calls=%d", calls)
	}
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("成功しても閉じない: %v", b.State())
	}
}

// 半開の試行が失敗すれば再び開く。
func TestHalfOpenReopensOnFailure(t *testing.T) {
	clock := newClock()
	b := circuitbreaker.New(settings(clock))
	fail(b, 10)
	clock.advance(30 * time.Second)

	_ = b.Do(func() error { return errFail })
	if b.State() != circuitbreaker.StateOpen {
		t.Errorf("失敗しても開放に戻らない: %v", b.State())
	}

	// 開放の起点が更新されていること。更新されないと即座に半開へ戻る。
	clock.advance(29 * time.Second)
	if b.State() != circuitbreaker.StateOpen {
		t.Errorf("開放の起点が更新されていない: %v", b.State())
	}
}

// 半開で通すのは1件だけ。回復途中の下流に一斉の負荷をかけない。
func TestHalfOpenAllowsOnlyOneTrial(t *testing.T) {
	clock := newClock()
	b := circuitbreaker.New(settings(clock))
	fail(b, 10)
	clock.advance(30 * time.Second)

	// 1件目を進行中のまま止め、その間に2件目を投げる。
	started := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = b.Do(func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	calls := 0
	err := b.Do(func() error { calls++; return nil })
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Errorf("半開で2件目が通った: err=%v", err)
	}
	if calls != 0 {
		t.Errorf("半開で2件目が下流を呼んだ: calls=%d", calls)
	}

	close(release)
	wg.Wait()
}

// 半開から閉じた直後に、開放前の失敗で即座に再開放しないこと。
func TestWindowIsResetOnStateChange(t *testing.T) {
	clock := newClock()
	b := circuitbreaker.New(settings(clock))
	fail(b, 10)
	clock.advance(30 * time.Second)

	// 半開の試行が成功して閉じる。
	_ = b.Do(func() error { return nil })

	// 1回失敗しただけで開放されるなら、開放前の失敗が残っている。
	fail(b, 1)
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("開放前の失敗が残っている: %v", b.State())
	}
}

// 設定の未指定項目が既定値で埋まること。
func TestZeroSettingsFallBackToDefaults(t *testing.T) {
	b := circuitbreaker.New(circuitbreaker.Settings{})

	fail(b, 9)
	if b.State() != circuitbreaker.StateClosed {
		t.Fatalf("既定の閾値が 10 でない: %v", b.State())
	}
	fail(b, 1)
	if b.State() != circuitbreaker.StateOpen {
		t.Errorf("既定の閾値で開放されない: %v", b.State())
	}
}

func TestStateString(t *testing.T) {
	for state, want := range map[circuitbreaker.State]string{
		circuitbreaker.StateClosed:   "closed",
		circuitbreaker.StateOpen:     "open",
		circuitbreaker.StateHalfOpen: "half_open",
		circuitbreaker.State(99):     "unknown",
	} {
		if got := state.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", state, got, want)
		}
	}
}

// 複数のゴルーチンから同時に使っても壊れないこと。
// api は Pod あたり多数のリクエストを並行処理する。
func TestConcurrentUse(t *testing.T) {
	b := circuitbreaker.New(settings(newClock()))

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Do(func() error {
				if i%2 == 0 {
					return errFail
				}
				return nil
			})
		}()
	}
	wg.Wait()
	// 状態は成否の並び順に依存するため断定しない。
	// ここで検出したいのは -race での競合である。
	_ = b.State()
}

// 状態の遷移が通知されること。
//
// 遮断器は「呼ばない」ことで働くため、遷移が観測できないと
// 静かに壊れているのか遮断しているのかを区別できない。
func TestOnStateChangeReportsTransitions(t *testing.T) {
	clock := newClock()
	var got []string

	s := settings(clock)
	s.OnStateChange = func(from, to circuitbreaker.State) {
		got = append(got, from.String()+"->"+to.String())
	}
	b := circuitbreaker.New(s)

	fail(b, 10) // closed -> open
	clock.advance(30 * time.Second)
	_ = b.Do(func() error { return nil }) // open -> half_open -> closed

	want := []string{"closed->open", "open->half_open", "half_open->closed"}
	if len(got) != len(want) {
		t.Fatalf("遷移の数が違う: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d 番目の遷移が違う: %s, want %s", i, got[i], want[i])
		}
	}
}

// 同じ状態への変更は通知しないこと。通知が増えると、変化に気づけなくなる。
func TestOnStateChangeIgnoresSameState(t *testing.T) {
	calls := 0
	s := settings(newClock())
	s.OnStateChange = func(circuitbreaker.State, circuitbreaker.State) { calls++ }
	b := circuitbreaker.New(s)

	// 閉じたまま成功を重ねる。close() は呼ばれないが、状態も変わらない。
	for range 5 {
		_ = b.Do(func() error { return nil })
	}
	if calls != 0 {
		t.Errorf("状態が変わっていないのに %d 回通知された", calls)
	}
}
