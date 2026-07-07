package vi

import (
	"fmt"

	"github.com/liamg/readline/pkg/engine"
)

type State struct {
	registers                   map[rune][]rune
	activeRegister              rune
	marks                       map[rune]int
	lastSearch                  []rune
	count                       int
	mode                        string
	lastCharSearchRune          rune
	lastCharSearchAction        *engine.Action
	lastCharSearchReverseAction *engine.Action
	lastChangeAction            *engine.Action
}

const defaultRegister = '0' // default to register 0

func NewState() *State {
	return &State{
		registers: make(map[rune][]rune),
		marks:     make(map[rune]int),
		mode:      ModeInsert,
	}
}

// Reset gets things ready for the next command
func (s *State) Reset() {
	s.activeRegister = 0
	s.count = 0
}

func (s *State) AddCountDigit(r rune) {
	if r < '0' || r > '9' {
		return
	}
	i := int(r - '0')
	if s.count == 0 {
		s.count = i
		return
	}
	s.count = (s.count * 10) + i
}

func (s *State) ResetCount() {
	s.count = 1
}

func (s *State) Count() int {
	if s.count == 0 {
		return 1
	}
	return s.count
}

func (s *State) ActiveRegister() rune {
	return s.activeRegister
}

func (s *State) SetActiveRegister(r rune) error {
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '"' || r == '-' || r == '+' || r == '*' || r == '/' {
		s.activeRegister = r
		return nil
	}
	return fmt.Errorf("invalid register %q", r)
}

func (s *State) SetMode(m string) {
	s.mode = m
}

func (s *State) GetMode() string {
	return s.mode
}

func (s *State) hasActiveRegister() bool {
	return s.activeRegister > 0
}

func (s *State) ReadActiveRegister() ([]rune, error) {
	if !s.hasActiveRegister() {
		return nil, fmt.Errorf("no active register")
	}
	r := s.activeRegister
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '"' || r == '-' {
		return s.registers[r], nil
	}

	switch r {
	case '/':
		return s.lastSearch, nil
	case '+':
		return nil, fmt.Errorf("cannot read from system clipboard")
	case '*':
		// TODO: primary selection (X11/Wayland middle-click buffer)
		return nil, fmt.Errorf("primary selection register not implemented yet")
	default:
		return nil, fmt.Errorf("invalid register %q", r)
	}
}

func (s *State) ReadPasteRegister() ([]rune, error) {
	if s.hasActiveRegister() {
		return s.ReadActiveRegister()
	}
	if value, ok := s.registers['"']; ok {
		return value, nil
	}
	return s.registers[defaultRegister], nil
}

func (s *State) WriteActiveRegister(value []rune) error {
	r := s.activeRegister
	if !s.hasActiveRegister() {
		r = defaultRegister
	}
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '"' || r == '-' {
		s.registers[r] = value
		return nil
	}

	switch r {
	case '/':
		return fmt.Errorf("cannot write to search register")
	case '+':
		return fmt.Errorf("cannot write to system clipboard")
	case '*':
		// TODO: primary selection (X11/Wayland middle-click buffer)
		return fmt.Errorf("primary selection register not implemented yet")
	default:
		return fmt.Errorf("invalid register %q", r)
	}
}
