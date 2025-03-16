package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mattn/go-tty"
)

const userHistory = ".gosh_user"
const cmdHistory = ".gosh_history"

var commandHistory []string
var historyIndex int

func main() {
	user := getUser()
	printFlag(user)

	loadHistory()

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
	if file, err := os.ReadFile(userHistory); err == nil {
		return strings.TrimSpace(string(file))
	}

	fmt.Print("Enter your name: ")
	reader := bufio.NewReader(os.Stdin)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	_ = os.WriteFile(userHistory, []byte(name), 0644)
	return name
}

func loadHistory() {
	if file, err := os.ReadFile(cmdHistory); err == nil {
		commandHistory = strings.Split(strings.TrimSpace(string(file)), "\n")
	}

	historyIndex = len(commandHistory)
}

func saveHistory() {
	if len(commandHistory) == 0 {
		return
	}
	_ = os.WriteFile(cmdHistory, []byte(strings.Join(commandHistory, "\n")), 0644)
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
		case 13: // enter key
			fmt.Println()
			return string(input), nil
		case 127, 8: //backspace key
			if cursorPos > 0 && len(input) > 0 { // Prevent deleting the prompt and check input length
				input = append(input[:cursorPos-1], input[cursorPos:]...) // Remove character at cursor position
				cursorPos--
				// Redraw the entire line to handle backspace properly
				clearLine()
				fmt.Print(prompt + string(input))
				// Move cursor back to the correct position
				if cursorPos < len(input) {
					fmt.Print("\033[" + fmt.Sprint(len(input)-cursorPos) + "D")
				}
			}
		case 9: // Tab key (Auto-completion)
			suggestion := autocomplete(string(input))
			if suggestion != "" && suggestion != string(input) {
				input = []rune(suggestion)
				cursorPos = len(input)
				clearLine()
				fmt.Print(prompt + string(input))
			}
		case 27: // Arrow keys (Up/Down for history)
			r2, _ := tty.ReadRune()
			if r2 == 91 {
				r3, _ := tty.ReadRune()
				if r3 == 65 { // up arrow
					if historyIndex > 0 {
						historyIndex--
						clearLine()
						input = []rune(commandHistory[historyIndex])
						cursorPos = len(input)
						fmt.Print(prompt + string(input))
					}
				} else if r3 == 66 { // Down arrow
					if historyIndex < len(commandHistory)-1 {
						historyIndex++
						clearLine()
						input = []rune(commandHistory[historyIndex])
						cursorPos = len(input)
						fmt.Print(prompt + string(input))
					} else {
						historyIndex = len(commandHistory)
						clearLine()
						input = nil
						cursorPos = 0
						fmt.Print(prompt)
					}
				}
			}
		default:
			if r != 0 {
				// Insert character at cursor position
				if cursorPos < len(input) {
					input = append(input[:cursorPos], append([]rune{r}, input[cursorPos:]...)...)
				} else {
					input = append(input, r)
				}
				cursorPos++
				fmt.Print(string(r))
			}
		}
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
		os.Exit(0)
	case "pwd":
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %v", err)
		}
		fmt.Println(dir)
		return nil
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Run through cmd.exe to handle built-in Windows commands
		cmd = exec.Command("cmd", "/c", input)
	} else {
		// Run normally on Unix-based systems
		cmd = exec.Command("sh", "-c", input)
	}

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
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
