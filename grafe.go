package main

import (
	"bytes"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"go.abhg.dev/goldmark/wikilink"

	mathjax "github.com/litao91/goldmark-mathjax"

	"go.abhg.dev/goldmark/anchor"

	"github.com/clarkmcc/go-typescript"
)

var templates map[string]*template.Template

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

func generateHtmlFile(markdownWriter goldmark.Markdown, sourceMd string, outputFile string) {
	var buf bytes.Buffer
	var err error

	context := parser.NewContext()
	err = markdownWriter.Convert([]byte(sourceMd), &buf, parser.WithContext(context))
	check(err)
	metaData := meta.Get(context)

	if metaData["Draft"] == true {
		return
	}

	createDirectoryPath(outputFile)
	file, err := os.Create(outputFile)
	check(err)
	defer file.Close()

	params := metaData["Params"]

	if params == nil {
		params = *new(map[any]any)
	}

	data := struct {
		Title      string
		Summary    string
		Body       template.HTML
		PageParams map[any]any
	}{
		Title:      metaData["Title"].(string),
		Summary:    metaData["Summary"].(string),
		Body:       template.HTML(buf.String()),
		PageParams: params.(map[any]any),
	}

	pageTemplateFile := addExtension(metaData["Template"].(string), ".html")

	pageTemplate, ok := templates[pageTemplateFile]
	if !ok {
		log.Fatalf("The template %s does not exist.\n", pageTemplateFile)
	}

	err = pageTemplate.ExecuteTemplate(file, pageTemplateFile, data)
	check(err)
}

func transpileTypescript(tsFilePath string, jsOutputPath string) {
	tsCode, err := os.ReadFile(tsFilePath)
	check(err)

	transpiled, err := typescript.TranspileString(string(tsCode))
	check(err)

	outputFile, err := os.Create(jsOutputPath)
	check(err)

	_, err = outputFile.WriteString(transpiled)
	check(err)
}

func main() {
	templates = make(map[string]*template.Template)

	templatesDir := "theme/templates/"

	layouts, err := filepath.Glob(templatesDir + "layouts/*")
	check(err)

	includes, err := filepath.Glob(templatesDir + "includes/*")
	check(err)

	for _, layout := range layouts {
		files := append(includes, layout)
		templates[filepath.Base(layout)] = template.Must(template.ParseFiles(files...))
	}

	markdownWriter := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			meta.Meta,
			extension.Table,
			&wikilink.Extender{},
			&anchor.Extender{
				Texter:   anchor.Text("#"),
				Position: anchor.Before,
			},
			mathjax.MathJax,
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

	err = os.RemoveAll("public")
	check(err)

	walk("content", func(fileName string) {
		if getExtension(fileName) == ".md" {
			fileData, err := os.ReadFile(fileName)
			check(err)
			generateHtmlFile(
				markdownWriter,
				string(fileData),
				"public/"+strings.TrimPrefix(
					changeExtension(fileName, ".html"),
					"content/",
				),
			)
		} else {
			newFileName := strings.TrimPrefix(fileName, "content/")
			createDirectoryPath("public/" + newFileName)
			copyFile(
				fileName,
				"public/"+newFileName,
			)
		}
	})

	walk("theme/static", func(fileName string) {
		newFileName := strings.TrimPrefix(fileName, "theme/static/")
		createDirectoryPath("public/" + newFileName)
		copyFile(
			fileName,
			"public/"+newFileName,
		)
	})

	walk("static", func(fileName string) {
		newFileName := strings.TrimPrefix(fileName, "static/")
		createDirectoryPath("public/" + newFileName)
		copyFile(
			fileName,
			"public/"+newFileName,
		)
	})

	walk("public", func(fileName string) {
		if getExtension(fileName) != ".ts" {
			return
		}
		newFileName := changeExtension(fileName, ".js")
		createDirectoryPath("public/" + newFileName)
		transpileTypescript(fileName, newFileName)
		err = os.Remove(fileName)
		check(err)
	})

	_, err = os.Create("public/.nojekyll")
	check(err)
}
