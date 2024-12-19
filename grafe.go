package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	fences "github.com/stefanfritsch/goldmark-fences"

	wikitable "github.com/movsb/goldmark-wiki-table"

	"go.abhg.dev/goldmark/wikilink"

	mathjax "github.com/litao91/goldmark-mathjax"

	"github.com/clarkmcc/go-typescript"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func walk(dir string, fileFunction func(filePath string)) {
	items, _ := os.ReadDir(dir)
	for _, item := range items {
		if !item.IsDir() {
			fileFunction(dir + "/" + item.Name())
		} else {
			walk(dir+"/"+item.Name(), fileFunction)
		}
	}
}

func getExtension(filePath string) string {
	filePathParts := strings.Split(strings.TrimSpace(filePath), "/")
	fileName := filePathParts[len(filePathParts)-1]
	fileNameElems := strings.Split(strings.TrimSpace(fileName), ".")
	extension := strings.TrimPrefix(fileName, fileNameElems[0])
	return extension
}

func removeExtension(filePath string) string {
	return strings.TrimSuffix(strings.TrimSpace(filePath), getExtension(filePath))
}

func addExtension(filePath string, newExtension string) string {
	return filePath + newExtension
}

func changeExtension(filePath string, newExtension string) string {
	return addExtension(removeExtension(filePath), newExtension)
}

func createDirectoryPath(path string) {
	err := os.MkdirAll(filepath.Dir(path), 0770)
	check(err)
}

func copyFile(sourcePath string, destinationPath string) {
	source, err := os.Open(sourcePath)
	check(err)
	defer source.Close()

	destination, err := os.Create(destinationPath)
	check(err)

	defer destination.Close()
	_, err = io.Copy(destination, source)
	check(err)
}

func generateHtmlFile(templates map[string]*template.Template, markdownWriter goldmark.Markdown, sourceMd string, outputFile string, config map[string]interface{}) {
	var buf bytes.Buffer
	var err error

	context := parser.NewContext()
	err = markdownWriter.Convert([]byte(sourceMd), &buf, parser.WithContext(context))
	check(err)
	metaData := meta.Get(context)

	if metaData["draft"] == true {
		return
	}

	createDirectoryPath(outputFile)
	file, err := os.Create(outputFile)
	check(err)
	defer file.Close()

	params := metaData["params"]

	if params == nil {
		params = *new(map[any]any)
	}

	data := struct {
		Title      string
		Summary    string
		Body       template.HTML
		PageParams any
		SiteParams any
	}{
		Body:       template.HTML(buf.String()),
		PageParams: metaData,
		SiteParams: config,
	}

	pageTemplateFile := addExtension(metaData["template"].(string), ".html")

	pageTemplate, ok := templates[pageTemplateFile]
	if !ok {
		log.Fatalf("The template %s does not exist.\n", pageTemplateFile)
	}

	err = pageTemplate.ExecuteTemplate(file, pageTemplateFile, data)
	check(err)
}

func transpileTypescriptFile(tsFilePath string, jsOutputPath string) {
	tsCode, err := os.ReadFile(tsFilePath)
	check(err)

	transpiled, err := typescript.TranspileString(string(tsCode))
	check(err)

	outputFile, err := os.Create(jsOutputPath)
	check(err)

	_, err = outputFile.WriteString(transpiled)
	check(err)
}

func generateTemplates(directory string) map[string]*template.Template {
	templates := make(map[string]*template.Template)

	templatesDir := directory

	layouts, err := filepath.Glob(templatesDir + "layouts/*")
	check(err)

	includes, err := filepath.Glob(templatesDir + "includes/*")
	check(err)

	for _, layout := range layouts {
		files := append(includes, layout)
		templates[filepath.Base(layout)] = template.Must(template.ParseFiles(files...))
	}

	return templates
}

func convertContentDirectory(templates map[string]*template.Template, markdownWriter goldmark.Markdown, config map[string]interface{}, ignoreObsidian bool) {
	walk("content", func(fileName string) {
		if getExtension(fileName) == ".md" && !strings.Contains(fileName, "IGNORE") {
			fileData, err := os.ReadFile(fileName)
			check(err)
			generateHtmlFile(
				templates,
				markdownWriter,
				string(fileData),
				"public/"+strings.TrimPrefix(
					changeExtension(fileName, ".html"),
					"content/",
				),
				config,
			)
		} else {
			if !strings.Contains(fileName, ".git") && !strings.Contains(fileName, "IGNORE") && !(ignoreObsidian && strings.Contains(fileName, ".obsidian")) {
				newFileName := strings.TrimPrefix(fileName, "content/")
				createDirectoryPath("public/" + newFileName)
				copyFile(
					fileName,
					"public/"+newFileName,
				)
			}
		}
	})
}

func readConfigFile(markdownWriter goldmark.Markdown, configFile string) map[string]interface{} {
	var buf bytes.Buffer

	fileData, err := os.ReadFile(configFile)
	check(err)

	context := parser.NewContext()
	err = markdownWriter.Convert([]byte(string(fileData)), &buf, parser.WithContext(context))
	check(err)

	return meta.Get(context)
}

func copyDirectoryFiles(directoryToCopy string, newDirectoryPath string) {
	walk(directoryToCopy, func(fileName string) {
		newFileName := strings.TrimPrefix(fileName, directoryToCopy)
		createDirectoryPath(newDirectoryPath + newFileName)
		copyFile(
			fileName,
			newDirectoryPath+newFileName,
		)
	})
}

func transpileTypescript(directory string) {
	walk(directory, func(fileName string) {
		if getExtension(fileName) != ".ts" {
			return
		}
		newFileName := changeExtension(fileName, ".js")
		createDirectoryPath(directory + "/" + newFileName)
		transpileTypescriptFile(fileName, newFileName)
		err := os.Remove(fileName)
		check(err)
	})
}

func startHTTPServer(directory string, port int) {
	fmt.Printf("Started server at http://localhost:%d/\n", port)
	http.Handle("/", http.FileServer(http.Dir(directory)))

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	closeHTTPServer := func() {
		httpServerExitDone.Done()
		httpServerExitDone.Wait()
		fmt.Println("\nClosed server\n")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		closeHTTPServer()
		os.Exit(1)
	}()

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func pruneDirectory(directory string) {
	err := os.RemoveAll(directory)
	check(err)
}

func main() {
	var err error

	enableTypeScriptTranspilationPtr := flag.Bool("transpile-ts", true, "Transpile all TypeScript in the `public` directory.")
	createNoJekyllFilePtr := flag.Bool("nojekyll", true, "Create `public/.nojekyll`; required to host static site on GitHub pages.")
	ignoreObsidianPtr := flag.Bool("ignoreobsidian", true, "Ignore .obsidian directory in content directory.")
	enableHttpServerPtr := flag.Bool("server", false, "Start HTTP server of `public` directory.")
	httpServerPortPtr := flag.Int("port", 8081, "Port at which to host HTTP server.")

	flag.Parse()

	pruneDirectory("public-generator")

	createDirectoryPath("public-generator/templates")
	copyDirectoryFiles("theme/templates", "public-generator/templates")
	copyDirectoryFiles("templates", "public-generator/templates")
	templates := generateTemplates("public-generator/templates/")

	configMarkdown := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	config := readConfigFile(configMarkdown, "config.md")

	markdownWriter := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithExtensions(
			meta.Meta,
			extension.Table,
			&wikilink.Extender{},
			mathjax.MathJax,
			extension.TaskList,
			extension.Table,
			&fences.Extender{},
			wikitable.New(),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(
					extension.NewTableHTMLRenderer(),
					500,
				),
			),
			html.WithUnsafe(),
		),
	)

	pruneDirectory("public")

	copyDirectoryFiles("theme/static", "public")

	copyDirectoryFiles("static", "public")

	convertContentDirectory(templates, markdownWriter, config, *ignoreObsidianPtr)

	pruneDirectory("public-generator")

	if *enableTypeScriptTranspilationPtr {
		transpileTypescript("public")
	}

	if *createNoJekyllFilePtr {
		_, err = os.Create("public/.nojekyll")
		check(err)
	}

	if *enableHttpServerPtr {
		startHTTPServer("public", *httpServerPortPtr)
	}
}
