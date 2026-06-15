// Package session は選択操作のライフサイクルを管理するステートマシンを提供する。
// ホットキーによる開始から、ラベルキー入力による領域の絞り込み、
// クリックの実行、そして待機状態への復帰までの一連の状態遷移を扱う。
package session

import (
	"fmt"
	"log"
	"sync"

	"github.com/tamutamu/keymouse/internal/spatial"
)

// State は選択セッションのライフサイクル上の状態を表す。
type State int

const (
	StateIdle      State = iota // ホットキー待ち
	StateSelecting              // ユーザーがラベルキーで絞り込み中
	StateExecuting              // クリック送信中
	StateCancelled              // Esc 押下。Idle へ戻る直前
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateSelecting:
		return "Selecting"
	case StateExecuting:
		return "Executing"
	case StateCancelled:
		return "Cancelled"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}

// Session はアクティブな選択状態のステートマシンを管理する。
// 公開メソッドはすべて単一ゴルーチン(メッセージループ)からの呼び出しを想定し、
// 念のため Mutex で保護する。
type Session struct {
	mu     sync.Mutex
	state  State
	action spatial.ClickAction

	cfg    spatial.Config
	labels []spatial.Key

	// schedule は各段で使うグリッド(列数, 行数)の並び。Start でモニターサイズから算出する。
	schedule [][2]int

	// stages は SelectionStage のスタック。
	stages []spatial.SelectionStage
}

// New は指定された設定とラベル集合で Session を生成する。
func New(cfg spatial.Config, labels []spatial.Key) *Session {
	return &Session{
		cfg:    cfg,
		labels: labels,
	}
}

// State は現在のセッション状態を返す。
func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// ClickAction は保留中のクリック種別を返す(Selecting/Executing 中に有効)。
func (s *Session) ClickAction() spatial.ClickAction {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.action
}

// Start は Idle から Selecting へ遷移する。クリック種別と表示領域(モニター矩形)を記録し、
// モニターサイズからグリッド予定表を算出して初期段を構築する。
// 既にセッションが進行中の場合は false を返す(多重起動防止)。
func (s *Session) Start(action spatial.ClickAction, displayRect spatial.Rect) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateIdle {
		log.Printf("session.Start: already in state %s, ignoring", s.state)
		return false
	}

	s.action = action
	s.schedule = spatial.GridSchedule(displayRect.W, displayRect.H, s.cfg)

	cols, rows := s.schedule[0][0], s.schedule[0][1]
	anchors := spatial.GenerateAnchors(cols, rows, displayRect, displayRect, s.labels)
	s.stages = append(s.stages[:0], spatial.SelectionStage{SearchArea: displayRect, Anchors: anchors})
	s.state = StateSelecting
	return true
}

// CurrentAnchors はスタック最上段のアンカー集合を返す(Selecting 以外では nil)。
func (s *Session) CurrentAnchors() []spatial.Anchor {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || len(s.stages) == 0 {
		return nil
	}
	return s.stages[len(s.stages)-1].Anchors
}

// CurrentDepth はスタックに積まれている段数を返す。
func (s *Session) CurrentDepth() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stages)
}

// SelectKey はラベルキー押下を処理する。キーが現在段のいずれかのアンカーに一致する場合:
//   - 最終段(予定表の末尾)に到達していれば (anchor, true, false) を返す。呼び出し側はクリックを実行する。
//   - そうでなければ次段をスタックに積み、(anchor, false, true) を返す。
//
// 有効なラベルでなければ (_, false, false) を返す。
func (s *Session) SelectKey(k spatial.Key) (anchor spatial.Anchor, shouldClick bool, advanced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || len(s.stages) == 0 {
		return spatial.Anchor{}, false, false
	}

	top := s.stages[len(s.stages)-1]
	for _, a := range top.Anchors {
		if a.Label == k {
			if len(s.stages) >= len(s.schedule) {
				// 最終段に到達 — クリックを実行する。
				s.state = StateExecuting
				return a, true, false
			}
			// 次段を積む。グリッドは予定表の次の要素を用いる。
			cols, rows := s.schedule[len(s.stages)][0], s.schedule[len(s.stages)][1]
			nextStage := spatial.SelectionStage{
				SearchArea: a.SourceArea,
				Anchors:    spatial.GenerateAnchors(cols, rows, a.DisplayRect, a.SourceArea, s.labels),
			}
			s.stages = append(s.stages, nextStage)
			return a, false, true
		}
	}
	return spatial.Anchor{}, false, false
}

// SelectKeyWithClick は Shift+キー押下を処理し、現在の段に関わらず一致アンカーで
// 即クリックする。一致すれば (anchor, true)、しなければ (zero, false) を返す。
func (s *Session) SelectKeyWithClick(k spatial.Key) (spatial.Anchor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || len(s.stages) == 0 {
		return spatial.Anchor{}, false
	}

	top := s.stages[len(s.stages)-1]
	for _, a := range top.Anchors {
		if a.Label == k {
			s.state = StateExecuting
			return a, true
		}
	}
	return spatial.Anchor{}, false
}

// Backspace は最上段をポップする。残り1段(これ以上戻れない)の場合は何もせず false を返す。
// 段をポップできた場合は true を返す。
func (s *Session) Backspace() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || len(s.stages) <= 1 {
		return false
	}
	s.stages = s.stages[:len(s.stages)-1]
	return true
}

// Cancel は Cancelled を経て即座に Idle へ戻す。
func (s *Session) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateIdle {
		return
	}
	s.state = StateCancelled
	s.resetLocked()
}

// FinishExecuting はクリック送信後にセッションを Idle へ戻す。
func (s *Session) FinishExecuting() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

// resetLocked はセッションを Idle に戻して状態をクリアする。呼び出し時は mu を保持していること。
func (s *Session) resetLocked() {
	s.state = StateIdle
	s.stages = s.stages[:0]
	s.action = 0
}
