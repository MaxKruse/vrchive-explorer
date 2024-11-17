package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/net/html"
)

type SearchResult struct {
	Name         string
	DownloadLink string
	SourceLink   string
}

var (
	dataFolder string
	searchText string
	resultList *tview.TextView
)

func searchFile(fileName string, searchText string, outputChannel chan SearchResult) {
	log.Println("Searching file", fileName, ", for", searchText)
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}

	ParseHTML(file, searchText, outputChannel)
}

func Search(outputChannel chan SearchResult) {
	searchTextCopy := searchText

	// find the html files
	files, err := os.ReadDir(dataFolder)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	// only use html files
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".html") {
			// search in the file
			searchFile(path.Join(dataFolder, file.Name()), searchTextCopy, outputChannel)
		}
	}

	close(outputChannel) // Close the channel after sending all results
}

func ProcessInput(app *tview.Application, inputField *tview.InputField) {
	// Clear the results and set initial text
	resultList.Clear()
	resultList.Write([]byte(fmt.Sprintf("Searching: '%s' ...", searchText)))

	resultChan := make(chan SearchResult)
	go Search(resultChan)

	startedSearching := true

	// Use a goroutine to handle results from the channel
	go func() {

		for {

			result, ok := <-resultChan
			if !ok {
				break
			}

			// Use app.QueueUpdateDraw to update the UI safely
			app.QueueUpdateDraw(func() {
				if startedSearching {
					resultList.Clear()
					startedSearching = false
				}

				resultStr := result.Name + "\t" + result.SourceLink + "\t" + result.DownloadLink + "\n"
				resultList.Write([]byte(resultStr))
			})
		}

		// Say when we're done
		app.QueueUpdateDraw(func() {
			resultList.Write([]byte("Done!"))

			inputField.SetText("")
			inputField.SetDisabled(false)
		})
	}()
}

func setupLogger(logFileName string) error {
	// Open or create the log file with append and write permissions
	logFile, err := os.OpenFile(logFileName, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}

	// Set the default logger to write to the file only
	log.SetOutput(logFile)

	return nil
}

func main() {
	// we need arguments for:
	// 1. the data folder of the html files, which defaults to ./data
	// The program will run as a TUI, where we are prompted to type a search term, and then the program will search the html files for that term and display the download + source links.

	// flags/arguments:
	// 1. data folder

	flag.StringVar(&dataFolder, "data", "./data", "The folder containing the html files to search.")

	flag.Parse()

	err := setupLogger("app.log")
	if err != nil {
		fmt.Println("Error setting up logger:", err)
		return
	}

	app := tview.NewApplication()

	defer func() {
		if r := recover(); r != nil {
			app.Stop()
			fmt.Println("Error:", r)
		}
	}()

	headerText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tview.Styles.PrimaryTextColor).
		SetText("VRChive Searcher")

	// Create an input field for user queries
	inputField := tview.NewInputField().
		SetLabel("Enter search: ").
		SetFieldWidth(60).
		SetPlaceholder("Type and press Enter...").
		SetChangedFunc(func(text string) {
			searchText = text
		})
	inputField.SetDoneFunc(func(key tcell.Key) {
		inputField.SetDisabled(true)
		ProcessInput(app, inputField)
	})

	// Create a scrollable list to display results
	resultList = tview.NewTextView()
	resultList.SetBorder(true)
	resultList.SetTextAlign(tview.AlignLeft)

	// Layout setup with a flexbox
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).      // Vertical layout
		AddItem(headerText, 2, 1, false). // Header occupies 3 rows
		AddItem(inputField, 3, 1, true).  // Input field occupies 3 rows
		AddItem(resultList, 0, 3, false)  // Result list occupies the remaining space

	log.Printf("Setup application and running it")

	// Start the app
	if err := app.SetRoot(layout, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}

}

// ParseHTML reads the HTML file and searches for matching content
func ParseHTML(r *os.File, searchText string, outChannel chan SearchResult) {
	// Read the content of the file
	content, err := os.ReadFile(r.Name())
	if err != nil {
		log.Printf("Error reading file %s: %v", r.Name(), err)
		return
	}

	// Parse the HTML content
	doc, err := html.Parse(strings.NewReader(string(content)))
	if err != nil {
		log.Printf("Error parsing HTML in file %s: %v", r.Name(), err)
		return
	}

	processAllItems(doc, outChannel)
}

func hasSearchText(text string, searchText string) bool {
	// do a case insensitive search
	return strings.Contains(strings.ToLower(text), strings.ToLower(searchText))
}

func hasClass(n *html.Node, className string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && attr.Val == className {
			return true
		}
	}

	return false
}

func isAnchor(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "a"
}

func isEm(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "em"
}

func isStrong(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "strong"
}

func processAllItems(doc *html.Node, outChannel chan SearchResult) {
	if doc.Type == html.ElementNode && doc.Data == "div" {
		if hasClass(doc, "chatlog__embed-text") {
			processNode(doc, outChannel)
		}
	}

	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		processAllItems(c, outChannel)
	}
}

func findNodetypeRecursively(node *html.Node, nodeType string) []*html.Node {
	var result []*html.Node

	if node.Type == html.ElementNode && node.Data == nodeType {
		result = append(result, node)
		return result
		// return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		result = append(result, findNodetypeRecursively(child, nodeType)...)
	}
	return result
}

func hasResurivelyText(node *html.Node, searchText string) bool {
	if node.Type == html.TextNode {
		if hasSearchText(node.Data, searchText) {
			return true
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasResurivelyText(child, searchText) {
			return true
		}
	}
	return false
}

func findNodeRecursively(node *html.Node, className string) *html.Node {
	if hasClass(node, className) {
		return node
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result := findNodeRecursively(child, className); result != nil {
			return result
		}
	}
	return nil
}

func processNode(doc *html.Node, outChannel chan SearchResult) {
	Result := SearchResult{}

	for child := range doc.ChildNodes() {
		if hasClass(child, "chatlog__embed-title") {
			// this has the title. in this div, there is another div that then has the text
			for textChild := range child.ChildNodes() {
				if hasClass(textChild, "chatlog__markdown chatlog__markdown-preserve") {
					textData := textChild.FirstChild.Data
					Result.Name = textData
				}
			}
		}

		log.Printf("Found title: %s", Result.Name)

		if hasClass(child, "chatlog__embed-description") || hasClass(child, "chatlog__embed-fields") && Result.Name != "" {
			// this has the description. its nested as hell, so we go slowly through it...

			linkContainer := findNodeRecursively(child, "chatlog__embed-fields")
			if linkContainer == nil {
				linkContainer = findNodeRecursively(child, "chatlog__embed-description")
			}

			log.Printf("Found linkContainer: %v", linkContainer)

			anchorTags := findNodetypeRecursively(linkContainer, "a")

			for _, aContainer := range anchorTags {
				hrefData := aContainer.Attr[0].Val
				log.Printf("Found Download link: %s on element %s", hrefData, aContainer.Data)

				// if the current container has a nested child anywhere that says Source, assign the source link
				if hasResurivelyText(aContainer, "Source") {
					Result.SourceLink = hrefData
				}

				// if the current container has a nested child anywhere that says Download, assign the download link
				if hasResurivelyText(aContainer, "Download") {
					Result.DownloadLink = hrefData
				}

			}
		}

		log.Printf("Found source: %s", Result.SourceLink)
		log.Printf("Found Download: %s", Result.DownloadLink)

	}

	// if the name, source or download dont contain the search text, we are done
	if hasSearchText(Result.Name, searchText) || hasSearchText(Result.SourceLink, searchText) || hasSearchText(Result.DownloadLink, searchText) {
		log.Printf("Result found for %s", searchText)
		log.Printf("Result: %v", Result)
		outChannel <- Result
		return
	} else {
		log.Printf("Cant find result for %s", searchText)
		log.Printf("Result: %v", Result)
	}
}
