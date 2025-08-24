package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	coalOffset     = 53
	carWidth       = 46
	carriageOffset = 81
)

type Config struct {
	Branch string
	Remote string
	Push   bool
	Force  bool
	Speed  int
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.BoolVar(&cfg.Push, "push", false, "Automatically push the code after the animation finishes (default: false)")
	flag.StringVar(&cfg.Remote, "remote", "origin", "Set the remote name")
	flag.StringVar(&cfg.Branch, "branch", "", "Set the local branch (default: current working branch)")
	flag.BoolVar(&cfg.Force, "force", false, "Force push the branch. WARNING: may overwrite history (default: false)")
	flag.IntVar(&cfg.Speed, "speed", 40, "Adjust animation speed — lower is faster")

	flag.Parse()
	return cfg
}

func main() {
	cfg := parseFlags()

	branch := cfg.Branch
	trainSpeed := cfg.Speed

	if branch == "" {
		var err error
		branch, err = getCurrentBranch()
		if err != nil {
			fmt.Println("Error: could not determine current branch:", err)
			return
		}
	}

	commits, err := getCommitInformation(branch, cfg.Remote)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if trainSpeed <= 0 {
		// Speed need to be a posative interger for ticker to work
		trainSpeed = 1
	}
	if trainSpeed > 120 {
		// Set a min speed for the train, technically can increase this but if you set too high you are
		// stuck waiting hours, and have to close the terminal
		trainSpeed = 120
	}

	frameDelay := time.Duration(trainSpeed) * time.Millisecond

	screen, err := tcell.NewScreen()
	if err != nil || screen.Init() != nil {
		fmt.Println("Failed to initialize screen:", err)
		return
	}
	defer screen.Fini()

	screen.HideCursor()
	screen.Clear()

	// Prevent Interrupt e.g. CTRL + C, Can remove but risks the terminal state being 'corrupted'
	signal.Ignore(os.Interrupt)

	cols, rows := screen.Size()
	bodyHeight := len(D51Body)

	trainX := cols
	trainY := rows/2 - bodyHeight/2

	ticker := time.NewTicker(frameDelay)
	defer ticker.Stop()

	trainLength := maxLength(len(commits))

	for trainX+trainLength >= 0 {
		screen.Clear()
		drawD51(screen, trainX, trainY, bodyHeight, commits)
		screen.Show()

		// wait for frame delay
		<-ticker.C
		// move train left by 1 column
		trainX--
	}

	screen.Clear()
	screen.Suspend()

	if cfg.Push {
		if err := pushToRemote(branch, cfg.Remote, cfg.Force); err != nil {
			fmt.Println("Push error:", err)
		}
	}
}

func maxLength(commitCount int) int {
	return len(D51Body[0]) + len(D51Coal[0]) + carWidth*commitCount + 10
}

func drawString(s tcell.Screen, trainX, trainY int, str string) {
	screenW, screenH := s.Size()

	if trainY < 0 || trainY >= screenH {
		return
	}
	for i, ch := range str {
		px := trainX + i
		if px >= 0 && px < screenW {
			s.SetContent(px, trainY, ch, nil, tcell.StyleDefault)
		}
	}
}

func drawD51(s tcell.Screen, trainX int, trainY int, bodyHeight int, commits []CommitInfo) {
	totalLength := maxLength(len(commits))

	for i, line := range D51Body {
		drawString(s, trainX, trainY+i, line)
	}

	wheelIndex := ((trainX + totalLength) / 3) % len(D51WHLs)
	if wheelIndex < 0 {
		wheelIndex += len(D51WHLs)
	}

	for i, line := range D51WHLs[wheelIndex] {
		drawString(s, trainX, trainY+bodyHeight+i, line)
	}

	for i, line := range D51Coal {
		drawString(s, trainX+coalOffset, trainY+i, line)
	}

	for i, commit := range commits {
		offset := carriageOffset + i*carWidth
		carriage := formatCarriage(commit)
		for j, line := range carriage {
			drawString(s, trainX+offset, trainY+j, line)
		}
	}
}

func formatCarriage(commit CommitInfo) []string {
	carriage := make([]string, len(D51Carriage))
	replacements := map[string]string{
		"{hash:^27}":          fmt.Sprintf("%-27s", formatCommitString(commit.Hash, 40)),
		"{msg:^27}":           fmt.Sprintf("%-27s", formatCommitString(commit.Message, 40)),
		"{Modifications:^27}": fmt.Sprintf("%-27s", formatCommitString(commit.Modifications, 40)),
	}

	for i, line := range D51Carriage {
		for placeholder, value := range replacements {
			line = strings.ReplaceAll(line, placeholder, value)
		}
		carriage[i] = line
	}
	return carriage
}

func formatCommitString(s string, width int) string {
	if len(s) > width {
		// truncate message if it longer than 40 char
		return s[:width-3] + "..."
	}
	return s + strings.Repeat(" ", width-len(s))
}
