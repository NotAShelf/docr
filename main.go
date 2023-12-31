package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"encoding/xml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Page represents a single page with its title, HTML content, and modification date.
type Page struct {
	Title            string
	Content          string
	ModificationDate time.Time
}

// Footer represents the footer section in the template.
type Footer struct {
	Year string
}

// Navbar represents the navigation bar section in the template.
type Navbar struct {
	Pages []Page
}

// Settings represents the configuration settings.
type Settings struct {
	GithubUsername         string `json:"githubUsername"`
	WebsiteName            string `json:"websiteName"`
	TemplateDir            string `json:"templateDir"`
	MarkdownDir            string `json:"markdownDir"`
	OutputDir              string `json:"outputDir"`
	WebsiteURL             string `json:"websiteURL"`
	WebsiteDescription     string `json:"websiteDescription"`
	TimestampsFromFilename bool   `json:"timestampsFromFilename"`
}

// RSSItem represents an individual item in the RSS feed.
type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type RSSChannel struct {
	XMLName     xml.Name  `xml:"channel"`
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []RSSItem `xml:"item"`
}

var log = logrus.New()

// checkDirectories checks if the directories specified in the settings exist.
func checkDirectories(settings Settings) {
	directories := []string{settings.TemplateDir, settings.MarkdownDir}
	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Fatalf("Error: %s directory does not exist", dir)
		}
	}
}

// generatePages traverses the specified directory, reads markdown files,
// converts them to HTML, and generates Page objects for each file.
func generatePages(dirPath string, timestampsFromFilename bool) ([]Page, error) {
	var pages []Page

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".md" && filepath.Base(path) != "README.md" {
			filename := strings.TrimSuffix(info.Name(), ".md")
			parts := strings.Split(filename, "-")

			if len(parts) >= 3 && len(parts[0]) == 4 && len(parts[1]) == 2 && len(parts[2]) == 2 {
				year, _ := strconv.Atoi(parts[0])
				month, _ := strconv.Atoi(parts[1])
				day, _ := strconv.Atoi(parts[2])
				var title string
				if len(parts) > 3 {
					title = strings.Join(parts[3:], "-")
				}

				timestamp := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				htmlContent := renderMarkdown(content)

				var page Page
				if title != "" {
					page.Title = fmt.Sprintf("%s-%s.html", timestamp.Format("2006-01-02"), title)
				} else {
					page.Title = fmt.Sprintf("%s.html", timestamp.Format("2006-01-02"))
					log.Warnf("Markdown file '%s' is missing a title. Using date as the title.", filepath.Base(path))
				}
				page.ModificationDate = timestamp
				page.Content = htmlContent

				pages = append(pages, page)
			} else {
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				htmlContent := renderMarkdown(content)

				var page Page
				page.Title = fmt.Sprintf("%s.html", filename)
				page.ModificationDate = info.ModTime()
				page.Content = htmlContent

				log.Warnf("Markdown file '%s' does not follow the correct naming format (yyyy-mm-dd-title.md). Using filename as title.", filepath.Base(path))

				pages = append(pages, page)
			}
		}

		return nil
	})

	return pages, err
}

// renderMarkdown converts the given Markdown content to HTML using goldmark.
func renderMarkdown(content []byte) string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table, extension.Footnote, extension.Linkify),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)

	var buf strings.Builder
	if err := md.Convert(content, &buf); err != nil {
		log.Fatal(err)
	}

	return buf.String()
}

// generateButtons generates the HTML buttons for each page (excluding index.html) in descending order of modification date.
func generateButtons(pages []Page) string {
	// Sort the pages by modification date in descending order
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].ModificationDate.After(pages[j].ModificationDate)
	})

	var buttons strings.Builder
	for _, page := range pages {
		if page.Title != "index" {
			button := fmt.Sprintf(`<a href="%s" class="button">%s</a>`, page.Title, page.ModificationDate.Format("2006-01-02"))
			buttons.WriteString(button)
		}
	}

	return buttons.String()
}

// copyStaticFiles copies the static files (CSS, JS, etc.) to the output directory.
func copyStaticFiles(outputDir string, templateDir string) error {
	// Source directories containing static files
	cssDir := filepath.Join(templateDir, "css")
	jsDir := filepath.Join(templateDir, "js")
	xslFile := filepath.Join(templateDir, "pretty-feed-v3.xsl")

	// Create the CSS directory in the output directory
	err := os.MkdirAll(filepath.Join(outputDir, "css"), os.ModePerm)
	if err != nil {
		return err
	}

	// Copy CSS files
	err = filepath.Walk(cssDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Read the CSS file
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Create the corresponding file in the output directory
			outputPath := filepath.Join(outputDir, "css", filepath.Base(path))
			err = os.WriteFile(outputPath, content, os.ModePerm)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Create the JS directory in the output directory
	err = os.MkdirAll(filepath.Join(outputDir, "js"), os.ModePerm)
	if err != nil {
		return err
	}

	// Copy JS files
	err = filepath.Walk(jsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Read the JS file
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Create the corresponding file in the output directory
			outputPath := filepath.Join(outputDir, "js", filepath.Base(path))
			err = os.WriteFile(outputPath, content, os.ModePerm)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Copy the pretty-feed-v3.xsl file to the output directory
	xslContent, err := os.ReadFile(xslFile)
	if err != nil {
		return err
	}

	xslOutputPath := filepath.Join(outputDir, "pretty-feed-v3.xsl")
	err = os.WriteFile(xslOutputPath, xslContent, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// generateRSS generates the RSS feed based on the provided pages.
func generateRSS(pages []Page, settings Settings) error {
	// Sort the pages by modification date in descending order
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].ModificationDate.After(pages[j].ModificationDate)
	})

	var rssItems []RSSItem
	for _, page := range pages {
		// Extract the title from the filename without the extension
		titleParts := strings.SplitN(page.Title, ".", 2)
		itemTitle := titleParts[0]

		// Check if the itemTitle follows the yyyy-mm-dd-title format
		parts := strings.SplitN(itemTitle, "-", 4)
		if len(parts) >= 3 && len(parts[0]) == 4 && len(parts[1]) == 2 && len(parts[2]) == 2 {
			itemTitle = strings.Join(parts[3:], "-")
		} else if len(parts) >= 2 && len(parts[0]) == 2 && len(parts[1]) == 2 {
			itemTitle = strings.Join(parts[2:], "-")
		}

		// If itemTitle is empty, use the date as the title
		if itemTitle == "" {
			itemTitle = page.ModificationDate.Format("2006-01-02")
		}

		item := RSSItem{
			Title:       itemTitle,
			Link:        itemTitle + ".html",
			Description: page.Content,
			PubDate:     page.ModificationDate.Format(time.RFC1123Z),
		}
		rssItems = append(rssItems, item)
	}

	channel := RSSChannel{
		Title:       settings.WebsiteName,
		Link:        settings.WebsiteURL,
		Description: settings.WebsiteDescription,
		Items:       rssItems,
	}

	rss := struct {
		XMLName xml.Name   `xml:"rss"`
		Version string     `xml:"version,attr"`
		Channel RSSChannel `xml:"channel"`
	}{
		Version: "2.0",
		Channel: channel,
	}

	xmlContent, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return err
	}

	// Create a buffer to hold the modified XML content
	xmlBuf := &bytes.Buffer{}

	// Write the XML processing instruction for the stylesheet
	stylesheetProcessingInstruction := fmt.Sprintf(`<?xml-stylesheet href="pretty-feed-v3.xsl" type="text/xsl"?>%s`, "\n")
	xmlBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	xmlBuf.WriteString(stylesheetProcessingInstruction)

	// Write the rest of the XML content
	xmlBuf.Write(xmlContent)

	rssFilePath := filepath.Join(settings.OutputDir, "rss.xml")
	err = os.WriteFile(rssFilePath, xmlBuf.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// generatePrettyFeedProcessingInstruction generates the XML processing instruction
// for the pretty-feed-v3.xsl stylesheet.
func generatePrettyFeedProcessingInstruction(prettyFeedPath string) string {
	return fmt.Sprintf(`<?xml-stylesheet href="%s" type="text/xsl"?>`, "pretty-feed-v3.xsl")
}

func initLogger() {
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})
}

func configureViper() {
	viper.SetConfigName("settings")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Bind environment variables
	viper.BindEnv("githubUsername", "DOCR_GITHUB_USERNAME")
	viper.BindEnv("websiteName", "DOCR_WEBSITE_NAME")
	viper.BindEnv("templateDir", "DOCR_TEMPLATE_DIR")
	viper.BindEnv("markdownDir", "DOCR_MARKDOWN_DIR")
	viper.BindEnv("outputDir", "DOCR_OUTPUT_DIR")
	viper.BindEnv("websiteURL", "DOCR_WEBSITE_URL")
	viper.BindEnv("websiteDescription", "DOCR_WEBSITE_DESCRIPTION")
	viper.BindEnv("timestampsFromFilename", "DOCR_TIMESTAMPS_FROM_FILENAME")

	if err := viper.ReadInConfig(); err != nil {
		log.Warnf("Failed to read configuration file: %v", err)
	} else {
		log.Info("Using configuration file:", viper.ConfigFileUsed())
	}
}

func main() {
	initLogger()
	configureViper()

	// Directory containing the markdown files
	dirPath := viper.GetString("markdownDir")

	// Output directory for generated HTML pages
	outputDir := viper.GetString("outputDir")

	// Template directory
	templateDir := viper.GetString("templateDir")

	// Load templates
	templates := template.Must(template.ParseGlob(filepath.Join(templateDir, "*.html")))

	// Generate pages
	pages, err := generatePages(dirPath, viper.GetBool("timestampsFromFilename"))
	if err != nil {
		log.Fatal(err)
	}

	// Create output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, os.ModePerm)
	}

	// Copy static files to output directory
	err = copyStaticFiles(outputDir, templateDir)
	if err != nil {
		log.Fatal(err)
	}

	// Load settings from Viper
	var settings Settings
	err = viper.Unmarshal(&settings)
	if err != nil {
		log.Fatal(err)
	}

	// Check if the directories in settings exist
	checkDirectories(settings)

	// Generate individual pages
	for _, page := range pages {
		// Extract the title from the page's title and remove .html extension
		pageTitle := strings.TrimSuffix(page.Title, ".html")

		// Check if the pageTitle follows the yyyy-mm-dd-title.md format
		parts := strings.SplitN(pageTitle, "-", 4)
		if len(parts) >= 3 && len(parts[0]) == 4 && len(parts[1]) == 2 && len(parts[2]) == 2 {
			pageTitle = strings.Join(parts[3:], "-")
		} else if len(parts) >= 2 && len(parts[0]) == 2 && len(parts[1]) == 2 {
			pageTitle = strings.Join(parts[2:], "-")
		}

		// If pageTitle is empty, use the date as the title
		if pageTitle == "" {
			pageTitle = page.ModificationDate.Format("2006-01-02")
		}

		// Create the output file
		pageFile, err := os.Create(filepath.Join(outputDir, page.Title)) // Remove ".html" from here
		if err != nil {
			log.Fatal(err)
		}
		defer pageFile.Close()

		// Combine the templates to generate the final HTML content for the page
		data := struct {
			Title            string
			Content          string
			GithubUsername   string
			WebsiteName      string
			Navbar           Navbar
			Footer           Footer
			ModificationDate string
		}{
			Title:            pageTitle, // Use extracted title without .html
			Content:          page.Content,
			GithubUsername:   settings.GithubUsername,
			WebsiteName:      settings.WebsiteName,
			Navbar:           Navbar{Pages: pages},
			Footer:           Footer{Year: "2023"}, // Update with the appropriate year
			ModificationDate: page.ModificationDate.Format(time.RFC1123),
		}

		err = templates.ExecuteTemplate(pageFile, "page.html", data)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Generated page: %s\n", page.Title) // Remove ".html" from here
	}

	// Read the README.md file
	readmeContent, err := os.ReadFile(filepath.Join(dirPath, "README.md"))
	if err != nil {
		log.Fatal(err)
	}

	// Convert README.md content to HTML
	readmeHTML := renderMarkdown(readmeContent)

	// Combine the templates to generate the final HTML content for the index page
	indexData := struct {
		WebsiteName                     string
		GithubUsername                  string
		ReadmeContent                   string
		Buttons                         string
		Navbar                          Navbar
		Footer                          Footer
		PrettyFeedProcessingInstruction string
	}{
		WebsiteName:                     settings.WebsiteName,
		GithubUsername:                  settings.GithubUsername,
		ReadmeContent:                   readmeHTML,
		Buttons:                         generateButtons(pages),
		Navbar:                          Navbar{Pages: pages},
		Footer:                          Footer{Year: "2023"}, // Update with the appropriate year
		PrettyFeedProcessingInstruction: generatePrettyFeedProcessingInstruction(filepath.Join(settings.TemplateDir, "pretty-feed-v3.xsl")),
	}

	// Create the index.html file
	indexFile, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		log.Fatal(err)
	}
	defer indexFile.Close()

	err = templates.ExecuteTemplate(indexFile, "index.html", indexData)
	if err != nil {
		log.Fatal(err)
	}

	// Generate RSS feed
	err = generateRSS(pages, settings)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Static pages and RSS feed generated successfully.")
}
