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
	bodyWidth      = 54
	coalWidth      = 30
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

	flag.BoolVar(&cfg.Push, "push", true, "Automatically push the code after the animation finishes (default: True)")
	flag.StringVar(&cfg.Remote, "remote", "origin", "Set the remote name (default: origin)")
	flag.StringVar(&cfg.Branch, "branch", "", "Set the local branch (default: current working branch)")
	flag.BoolVar(&cfg.Force, "force", false, "Force push the branch. USE WITH CAUTION (default: false)")
	flag.IntVar(&cfg.Speed, "speed", 40, "Adjust animation tick speed - lower is faster (default: 40)")

	flag.Parse()
	return cfg
}

func main() {
	cfg := parseFlags()

	branch := cfg.Branch
	trainSpeed := cfg.Speed

	// Determine the branch to use
	if branch == "" {
		var err error
		branch, err = getCurrentBranch()
		if err != nil {
			fmt.Println("Error: could not determine current branch:", err)
			return
		}
	}

	// Retrieve commit information for the animation
	commits, err := getCommitInformation(branch, cfg.Remote)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	carriages := make([][]string, len(commits))
	for i, commit := range commits {
		carriages[i] = formatCarriage(commit)
	}

	// Validate and clamp animation speed
	if trainSpeed <= 0 {
		// Speed need to be a positive integer for ticker to work
		trainSpeed = 1
	}
	if trainSpeed > 120 {
		// Set a max speed for the train, technically can increase this but if you set too high you are
		// stuck waiting hours, and have to close the terminal
		trainSpeed = 120
	}

	tickSpeed := time.Duration(trainSpeed) * time.Millisecond

	// Initialize the terminal screen for animation
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

	ticker := time.NewTicker(tickSpeed)
	defer ticker.Stop()

	trainLength := maxLength(len(commits))

	// Animation loop: move train from right to left
	for trainX+trainLength >= 0 {
		screen.Clear()
		drawTrain(screen, trainX, trainY, bodyHeight, carriages)
		screen.Show()

		// wait for frame delay
		<-ticker.C
		// move train left by 1 column
		trainX--
	}

	screen.Clear()
	screen.Suspend()

	// Push to remote if configured
	if cfg.Push {
		if err := pushToRemote(branch, cfg.Remote, cfg.Force); err != nil {
			fmt.Println("Push error:", err)
		}
	}
}

func maxLength(commitCount int) int {
	return bodyWidth + coalWidth + carWidth*commitCount + 10
}

// Renders the given string on the screen starting at the specified position.
// It clips the string to the screen boundaries.
func renderText(scr tcell.Screen, trainX, trainY int, str string) {
	screenW, screenH := scr.Size()

	if trainY < 0 || trainY >= screenH {
		return
	}
	for i, ch := range str {
		px := trainX + i
		if px >= 0 && px < screenW {
			scr.SetContent(px, trainY, ch, nil, tcell.StyleDefault)
		}
	}
}

func drawTrain(scr tcell.Screen, trainX int, trainY int, bodyHeight int, carriages [][]string) {
	totalLength := maxLength(len(carriages))

	for i, line := range D51Body {
		renderText(scr, trainX, trainY+i, line)
	}

	wheelIndex := ((trainX + totalLength) / 3) % len(D51WHLs)
	if wheelIndex < 0 {
		wheelIndex += len(D51WHLs)
	}

	for i, line := range D51WHLs[wheelIndex] {
		renderText(scr, trainX, trainY+bodyHeight+i, line)
	}

	for i, line := range D51Coal {
		renderText(scr, trainX+coalOffset, trainY+i, line)
	}

	for i, carriage := range carriages {
		offset := carriageOffset + i*carWidth
		for j, line := range carriage {
			renderText(scr, trainX+offset, trainY+j, line)
		}
	}
}

func formatCarriage(commit CommitInfo) []string {
	carriage := make([]string, len(D51Carriage))
	replacements := map[string]string{
		"{hash}":          fmt.Sprintf("%-27s", formatCommitString(commit.Hash, 40)),
		"{msg}":           fmt.Sprintf("%-27s", formatCommitString(commit.Message, 40)),
		"{modifications}": fmt.Sprintf("%-27s", formatCommitString(commit.Modifications, 40)),
	}

	for i, line := range D51Carriage {
		for placeholder, value := range replacements {
			line = strings.ReplaceAll(line, placeholder, value)
		}
		carriage[i] = line
	}
	return carriage
}

func formatCommitString(input string, width int) string {
	if len(input) > width {
		// truncate message if it longer than 40 char
		return input[:width-3] + "..."
	}
	return input + strings.Repeat(" ", width-len(input))
}
