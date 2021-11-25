package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/tonistiigi/vt100"
	"go.dagger.io/dagger/environment"
)

type Event map[string]interface{}

type Group struct {
	Name      string
	State     environment.State
	Events    []Event
	Started   *time.Time
	Completed *time.Time
}

type Message struct {
	Event Event
	Group *Group
}

type Logs struct {
	Messages []Message

	groups map[string]*Group
	l      sync.Mutex
}

func (l *Logs) Add(event Event) error {
	l.l.Lock()
	defer l.l.Unlock()

	task, ok := event["task"].(string)
	if !ok {
		l.Messages = append(l.Messages, Message{
			Event: event,
		})

		return nil
	}

	groupKey := strings.Split(task, ".#up")[0]
	group := l.groups[groupKey]

	// If the group doesn't exist, create it
	if group == nil {
		now := time.Now()
		group = &Group{
			Name:    groupKey,
			Started: &now,
		}
		l.groups[groupKey] = group
		l.Messages = append(l.Messages, Message{
			Group: group,
		})
	}

	// Handle state events
	// For state events, we just want to update the group status -- no need to
	// dispanything
	if st, ok := event["state"].(string); ok {
		// Ignore state updates for "sub" tasks
		if task != groupKey {
			return nil
		}

		group.State = environment.State(st)
		if group.State != environment.StateComputing {
			now := time.Now()
			group.Completed = &now
		}

		return nil
	}

	group.Events = append(group.Events, event)

	return nil
}

type TTYOutput struct {
	cons      console.Console
	logs      *Logs
	lineCount int
	l         sync.RWMutex

	stopCh  chan struct{}
	doneCh  chan struct{}
	printCh chan struct{}
}

func NewTTYOutput(w *os.File) (*TTYOutput, error) {
	cons, err := console.ConsoleFromFile(w)
	if err != nil {
		return nil, err
	}

	c := &TTYOutput{
		logs: &Logs{
			groups: make(map[string]*Group),
		},
		cons:    cons,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		printCh: make(chan struct{}, 128),
	}

	return c, nil
}

func (c *TTYOutput) Start() {
	defer close(c.doneCh)
	go func() {
		for {
			select {
			case <-c.stopCh:
				return
			case <-c.printCh:
				c.print()
			case <-time.After(100 * time.Millisecond):
				c.print()
			}
		}
	}()
}

func (c *TTYOutput) Stop() {
	c.l.Lock()
	defer c.l.Unlock()

	if c.doneCh == nil {
		return
	}
	close(c.stopCh)
	<-c.doneCh
	c.doneCh = nil
}

func (c *TTYOutput) Write(p []byte) (n int, err error) {
	event := Event{}
	d := json.NewDecoder(bytes.NewReader(p))
	if err := d.Decode(&event); err != nil {
		return n, fmt.Errorf("cannot decode event: %s", err)
	}

	if err := c.logs.Add(event); err != nil {
		return 0, err
	}

	c.print()

	return len(p), nil
}

func (c *TTYOutput) print() {
	c.l.Lock()
	defer c.l.Unlock()

	// make sure the printer is not stopped
	select {
	case <-c.stopCh:
		return
	default:
	}

	width, height := c.getSize()

	// hide during re-rendering to avoid flickering
	fmt.Fprint(c.cons, aec.Hide)
	defer fmt.Fprint(c.cons, aec.Show)

	// rewind to the top
	b := aec.EmptyBuilder
	for i := 0; i < c.lineCount; i++ {
		b = b.Up(1)
	}
	fmt.Fprint(c.cons, b.ANSI)

	linesPerGroup := c.linesPerGroup(width, height)
	lineCount := 0
	for _, message := range c.logs.Messages {
		if group := message.Group; group != nil {
			lineCount += c.printGroup(group, width, linesPerGroup)
		} else {
			lineCount += c.printLine(c.cons, message.Event, width)
		}
	}

	if diff := c.lineCount - lineCount; diff > 0 {
		for i := 0; i < diff; i++ {
			fmt.Fprintln(c.cons, strings.Repeat(" ", width))
		}
		fmt.Fprint(c.cons, aec.EmptyBuilder.Up(uint(diff)).Column(0).ANSI)
	}

	c.lineCount = lineCount
}

func (c *TTYOutput) linesPerGroup(width, height int) int {
	usedLines := 0
	for _, message := range c.logs.Messages {
		if group := message.Group; group != nil {
			usedLines++
			continue
		}
		usedLines += c.printLine(io.Discard, message.Event, width)
	}

	runningGroups := 0
	for _, message := range c.logs.Messages {
		if group := message.Group; group != nil && group.State == environment.StateComputing {
			runningGroups++
		}
	}

	linesPerGroup := 5
	if freeLines := (height - usedLines); freeLines > 0 && runningGroups > 0 {
		linesPerGroup = (freeLines - 2) / runningGroups
	}

	return linesPerGroup
}

func (c *TTYOutput) printLine(w io.Writer, event Event, width int) int {
	message := colorize.Color(fmt.Sprintf("%s %s %s%s",
		formatTimestamp(event),
		formatLevel(event),
		formatMessage(event),
		formatFields(event),
	))

	// pad
	if delta := width - utf8.RuneCountInString(message); delta > 0 {
		message += strings.Repeat(" ", delta)
	}
	message += "\n"

	// print
	fmt.Fprint(w, message)

	t := vt100.NewVT100(100, width)
	t.Write([]byte(message))
	return t.UsedHeight()
}

func (c *TTYOutput) printGroup(group *Group, width, maxLines int) int {
	lineCount := 0

	prefix := ""
	switch group.State {
	case environment.StateComputing:
		prefix = "[+]"
	case environment.StateCanceled:
		prefix = "[✗]"
	case environment.StateFailed:
		prefix = "[✗]"
	case environment.StateCompleted:
		prefix = "[✔]"
	}

	out := prefix + " " + group.Name

	endTime := time.Now()
	if group.Completed != nil {
		endTime = *group.Completed
	}

	dt := endTime.Sub(*group.Started).Seconds()
	if dt < 0.05 {
		dt = 0
	}
	timer := fmt.Sprintf("%3.1fs", dt)

	// align
	out += strings.Repeat(" ", width-utf8.RuneCountInString(out)-len(timer))
	out += timer
	out += "\n"

	// color
	switch group.State {
	case environment.StateComputing:
		out = aec.Apply(out, aec.LightBlueF)
	case environment.StateCanceled:
		out = aec.Apply(out, aec.LightYellowF)
	case environment.StateFailed:
		out = aec.Apply(out, aec.LightRedF)
	case environment.StateCompleted:
		out = aec.Apply(out, aec.LightGreenF)
	}

	// Print
	fmt.Fprint(c.cons, out)
	lineCount++

	printEvents := []Event{}
	switch group.State {
	case environment.StateComputing:
		printEvents = group.Events
		// for computing tasks, show only last N
		if len(printEvents) > maxLines {
			printEvents = printEvents[len(printEvents)-maxLines:]
		}
	case environment.StateCanceled:
		// for completed tasks, don't show any logs
		printEvents = []Event{}
	case environment.StateFailed:
		// for failed, show all logs
		printEvents = group.Events
	case environment.StateCompleted:
		// for completed tasks, don't show any logs
		printEvents = []Event{}
	}

	for _, event := range printEvents {
		lineCount += c.printGroupLine(event, width)
	}

	return lineCount
}

func (c *TTYOutput) printGroupLine(event Event, width int) int {
	message := colorize.Color(fmt.Sprintf("%s%s",
		formatMessage(event),
		formatFields(event),
	))

	// trim
	for utf8.RuneCountInString(message) > width {
		message = message[0:len(message)-4] + "…"
	}

	// pad
	if delta := width - utf8.RuneCountInString(message); delta > 0 {
		message += strings.Repeat(" ", delta)
	}
	message += "\n"

	// color
	message = aec.Apply(message, aec.Faint)

	// Print
	fmt.Fprint(c.cons, message)

	return 1
}

func (c *TTYOutput) getSize() (int, int) {
	width := 80
	height := 10
	size, err := c.cons.Size()
	if err == nil && size.Width > 0 && size.Height > 0 {
		width = int(size.Width)
		height = int(size.Height)
	}

	return width, height
}
