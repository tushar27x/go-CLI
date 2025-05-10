package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/mattn/go-tty"
)

const (
	userHistory = ".gosh_user"
	cmdHistory  = ".gosh_history"
)

var (
	commandHistory []string
	historyIndex   int
	homeDir        string
)

func init() {
	var err error
	homeDir, err = os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		os.Exit(1)
	}
}

func main() {
	user := getUser()
	printFlag(user)

	loadHistory()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		for range signalChan {
			fmt.Println()
			fmt.Print(getPrompt())
		}
	}()
	tty, err := tty.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening TTY:", err)
		return
	}

	defer tty.Close()

	for {
		input, err := readInputWithHistory(tty)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		commandHistory = append(commandHistory, input)
		historyIndex = len(commandHistory)

		saveHistory()

		err = execInput(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
	}
}

func getUser() string {
	userFile := filepath.Join(homeDir, userHistory)
	if file, err := os.ReadFile(userFile); err == nil {
		return strings.TrimSpace(string(file))
	}

	fmt.Print("Enter your name: ")
	reader := bufio.NewReader(os.Stdin)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	_ = os.WriteFile(userFile, []byte(name), 0644)
	return name
}

func loadHistory() {
	historyFile := filepath.Join(homeDir, cmdHistory)
	if file, err := os.ReadFile(historyFile); err == nil {
		commandHistory = strings.Split(strings.TrimSpace(string(file)), "\n")
	}

	historyIndex = len(commandHistory)
}

func saveHistory() {
	if len(commandHistory) == 0 {
		return
	}
	historyFile := filepath.Join(homeDir, cmdHistory)
	_ = os.WriteFile(historyFile, []byte(strings.Join(commandHistory, "\n")), 0644)
}

func readInputWithHistory(tty *tty.TTY) (string, error) {
	var input []rune
	prompt := getPrompt()
	fmt.Print(prompt)
	cursorPos := 0

	for {
		r, err := tty.ReadRune()
		if err != nil {
			return strings.TrimSpace(string(input)), err
		}

		switch r {
		case 13: // Enter key
			fmt.Println()
			return string(input), nil
		case 127, 8: // Backspace key
			if cursorPos > 0 && len(input) > 0 {
				input = append(input[:cursorPos-1], input[cursorPos:]...)
				cursorPos--
				redrawLine(prompt, input, cursorPos)
			}
		case 9: // Tab key (Auto-completion)
			suggestion := autocomplete(string(input))
			if suggestion != "" && suggestion != string(input) {
				input = []rune(suggestion)
				cursorPos = len(input)
				redrawLine(prompt, input, cursorPos)
			}
		case 27: // Escape sequence (Arrow keys)
			r2, _ := tty.ReadRune()
			if r2 == 91 { // '[' character
				r3, _ := tty.ReadRune()
				switch r3 {
				case 65: // Up arrow (History back)
					if historyIndex > 0 {
						historyIndex--
						input = []rune(commandHistory[historyIndex])
						cursorPos = len(input)
						redrawLine(prompt, input, cursorPos)
					}
				case 66: // Down arrow (History forward)
					if historyIndex < len(commandHistory)-1 {
						historyIndex++
						input = []rune(commandHistory[historyIndex])
					} else {
						historyIndex = len(commandHistory)
						input = nil
					}
					cursorPos = len(input)
					redrawLine(prompt, input, cursorPos)
				case 67: // Right arrow (Move cursor right)
					if cursorPos < len(input) {
						cursorPos++
						fmt.Print("\033[C") // Move cursor forward
					}
				case 68: // Left arrow (Move cursor left)
					if cursorPos > 0 {
						cursorPos--
						fmt.Print("\033[D")
					}
				}
			}
		default:
			if r != 0 {
				if cursorPos < len(input) {
					input = append(input[:cursorPos], append([]rune{r}, input[cursorPos:]...)...)
				} else {
					input = append(input, r)
				}
				cursorPos++
				redrawLine(prompt, input, cursorPos)
			}
		}
	}
}

func redrawLine(prompt string, input []rune, cursorPos int) {
	fmt.Print("\r\033[K")
	fmt.Print(prompt + string(input))
	if cursorPos < len(input) {
		fmt.Printf("\033[%dD", len(input)-cursorPos)
	}
}

func execInput(input string) error {
	args := strings.Fields(input)
	if len(args) == 0 {
		return nil
	}

	switch args[0] {
	case "cd":
		if len(args) < 2 {
			return errors.New("path required for cd")
		}
		if err := os.Chdir(args[1]); err != nil {
			return fmt.Errorf("failed to change directory: %v", err)
		}
		return nil
	case "exit":
		fmt.Println("Closing shell...")
		// Clear both in-memory history and history file
		commandHistory = nil
		historyIndex = 0
		historyFile := filepath.Join(homeDir, cmdHistory)
		_ = os.WriteFile(historyFile, []byte(""), 0644)

		// 🛑 Detect OS and close terminal
		if isWindows() {
			exec.Command("cmd", "/c", "exit").Run()
		} else {
			exec.Command("bash", "-c", "exit").Run()
		}
		os.Exit(0) // Ensure Go process exits

	case "pwd":
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %v", err)
		}
		fmt.Println(dir)
		return nil
	case "ls":
		// Cross-platform support for 'ls'
		if isWindows() {
			args = []string{"cmd", "/c", "dir"} // Windows uses 'dir'
		} else {
			args = []string{"ls", "-la"} // Unix-based OS
		}
	case "clear", "cls": // Handle clearing the screen
		fmt.Print("\033[H\033[2J") // ANSI escape sequence to clear screen
		return nil
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

func printFlag(user string) {
	fmt.Println(`
#   /$$   /$$           /$$ /$$                 /$$      /$$                     /$$       /$$       /$$
#  | $$  | $$          | $$| $$                | $$  /$ | $$                    | $$      | $$      | $$
#  | $$  | $$  /$$$$$$ | $$| $$  /$$$$$$       | $$ /$$$| $$  /$$$$$$   /$$$$$$ | $$  /$$$$$$$      | $$
#  | $$$$$$$$ /$$__  $$| $$| $$ /$$__  $$      | $$/$$ $$ $$ /$$__  $$ /$$__  $$| $$ /$$__  $$      | $$
#  | $$__  $$| $$$$$$$$| $$| $$| $$  \ $$      | $$$$_  $$$$| $$  \ $$| $$  \__/| $$| $$  | $$      |__/
#  | $$  | $$| $$_____/| $$| $$| $$  | $$      | $$$/ \  $$$| $$  | $$| $$      | $$| $$  | $$          
#  | $$  | $$|  $$$$$$$| $$| $$|  $$$$$$/      | $$/   \  $$|  $$$$$$/| $$      | $$|  $$$$$$$       /$$
#  |__/  |__/ \_______/|__/|__/ \______/       |__/     \__/ \______/ |__/      |__/ \_______/      |__/
#                                                                                                       
#                                                                                                       
#                                                                                                       
`)
	fmt.Printf("🔥 Welcome to your personal Go Shell, %s! 🔥", user)
	fmt.Println("Type 'exit' to quit.")
	fmt.Println()
}

func getPrompt() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("\033[1;34m%s\033[0m > ", dir)
}

func autocomplete(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return input
	}

	if len(parts) == 1 && !strings.Contains(parts[0], "/") {
		commands := []string{"cd", "exit", "pwd", "ls", "cat", "echo", "grep", "mkdir", "rm", "cp", "mv"}
		var matches []string

		for _, cmd := range commands {
			if strings.HasPrefix(cmd, parts[0]) {
				matches = append(matches, cmd)
			}
		}

		if len(matches) == 1 {
			return matches[0]
		} else if len(matches) > 1 {
			printMatches(matches)
			fmt.Print(getPrompt() + input)
			return input
		}
	}

	// Path autocomplete (for cd and other commands)
	lastWord := parts[len(parts)-1]

	// Determine if we're dealing with a path
	if lastWord == "" || !strings.Contains(lastWord, "/") {
		// Simple filename in current directory
		dirToSearch := "."
		filePrefix := lastWord

		entries, err := os.ReadDir(dirToSearch)
		if err != nil {
			return input
		}

		var matches []string
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), filePrefix) {
				match := entry.Name()
				if entry.IsDir() {
					match += "/"
				}
				matches = append(matches, match)
			}
		}

		if len(matches) == 0 {
			return input
		}

		if len(matches) == 1 {
			parts[len(parts)-1] = matches[0]
			return strings.Join(parts, " ")
		}

		printMatches(matches)
		fmt.Print(getPrompt() + input)
		return input
	} else {

		originalPath := lastWord
		hasDotSlash := strings.HasPrefix(originalPath, "./")
		hasDotDotSlash := strings.HasPrefix(originalPath, "../")

		var dirToSearch string
		var filePrefix string
		var pathPrefix string

		if hasDotSlash || hasDotDotSlash {
			if hasDotSlash {
				pathPrefix = "./"
			} else {
				pathPrefix = "../"
			}

			trimmedPath := strings.TrimPrefix(originalPath, pathPrefix)
			if strings.Contains(trimmedPath, "/") {
				dirToSearch = pathPrefix + filepath.Dir(trimmedPath)
				filePrefix = filepath.Base(trimmedPath)
			} else {
				dirToSearch = pathPrefix
				filePrefix = trimmedPath
			}
		} else {
			dirToSearch = filepath.Dir(originalPath)
			filePrefix = filepath.Base(originalPath)
		}

		readDir := dirToSearch
		if readDir == "./" {
			readDir = "."
		} else if readDir == "../" {
			readDir = ".."
		}

		entries, err := os.ReadDir(readDir)
		if err != nil {
			return input
		}

		var matches []string
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), filePrefix) {
				var matchPath string
				if dirToSearch == "." {
					matchPath = entry.Name()
				} else if dirToSearch == "./" {
					matchPath = "./" + entry.Name()
				} else if dirToSearch == "../" {
					matchPath = "../" + entry.Name()
				} else {
					matchPath = filepath.Join(dirToSearch, entry.Name())
				}

				if entry.IsDir() {
					matchPath += "/"
				}

				matches = append(matches, matchPath)
			}
		}

		if len(matches) == 0 {
			return input
		}

		if len(matches) == 1 {
			parts[len(parts)-1] = matches[0]
			return strings.Join(parts, " ")
		}

		printMatches(matches)
		fmt.Print(getPrompt() + input)
		return input
	}
}

func printMatches(matches []string) {
	fmt.Println()
	columnWidth := 20
	for i, match := range matches {
		fmt.Printf("%-*s", columnWidth, match)
		if (i+1)%4 == 0 {
			fmt.Println()
		}
	}
	if len(matches)%4 != 0 {
		fmt.Println()
	}
}

func clearLine() {
	fmt.Print("\r\033[K")
}
