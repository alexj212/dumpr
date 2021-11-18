// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"embed"
	"fmt"
	"github.com/potakhov/loge"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed web/*
var webEmbedFS embed.FS

// SetupFS set up FS reference, will check if local disk version of assets exists, or will use the embedded assets if the local asset dir does not contain the same file names
func SetupFS() (fs.FS, error) {
	var webFS fs.FS
	var root fs.FS

	webFS, _ = fs.Sub(webEmbedFS, "web")

	if *webDir != "" && !*exportTemplates {
		fi, err := os.Stat(*webDir)
		if err !=nil {
			loge.Info("unable to serve web assets from %d dir %v\n", *webDir, err)
		}

		if err == nil && fi.IsDir() {

			file := os.DirFS(*webDir)

			if file != nil {
				invalidFiles := CompareFS(webFS, file)
				fmt.Printf("Compared FS - found %d diffs\n", len(invalidFiles))

				if len(invalidFiles) > 0 {
					return nil, fmt.Errorf("compared fs - found %d diffs", len(invalidFiles))
				}

				root = file
				loge.Info("using file serving from local disk: %v\n", file)
				_ = walkDir(file, "local")
			}
		} else {
			loge.Info("unable to serve web assets from local dir %v\n", *webDir)
		}
	}

	if root == nil {
		loge.Info("using file serving from packed resources \n")
		root = webFS
		_ = walkDir(webFS, "static")
	}

	return root, nil

}

func walkDir(root fs.FS, fsName string) (err error) {

	err = fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			fmt.Printf("%s Assets path=%q\n", fsName, path)
		}

		return nil
	})
	return
}

// CompareFS compare 2 FS and will return a list of files that do not exist in the srcFS
func CompareFS(srcFS fs.FS, destFS fs.FS) []string {

	invalidList := make([]string, 0)

	_ = fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fi, err := destFS.Open(path)
		if err != nil || fi == nil {
			invalidList = append(invalidList, path)
			fmt.Printf("CompareFS path=%v does not exist\n", path)
		}
		return nil
	})
	return invalidList
}

func copyTemplatesToTarget(target string) (err error) {

	err = os.MkdirAll(target, 0777)
	if err != nil {
		return
	}

	err = SaveAssets(target, webEmbedFS, false)
	if err != nil {
		return err
	}
	return nil
}

// SaveAssets will save the prepacked templates for local editing. File structure will be recreated under the output dir.
func SaveAssets(outputDir string, srcFS embed.FS, overwrite bool) (err error) {
	if outputDir == "" {
		outputDir = "."
	}

	if strings.HasSuffix(outputDir, "/") {
		outputDir = outputDir[:len(outputDir)-1]
	}

	if outputDir == "" {
		outputDir = "."
	}

	err = fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		fileName := fmt.Sprintf("%s/%s", outputDir, d.Name())
		if d.IsDir() {
		} else {
			f, err := srcFS.Open(path)
			if err != nil {
				return err
			}

			err = WriteNewFile(fileName, f)
			if err != nil {
				return err
			}

		}
		return nil
	})

	return err
}

// WriteNewFile will attempt to write a file with the filename and path, a Reader and the FileMode of the file to be created.
// If an error is encountered an error will be returned.
func WriteNewFile(fpath string, in io.Reader) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0775)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}

	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("%s: creating new file: %v", fpath, err)
	}
	defer func() {
		_ = out.Close()
	}()

	fmt.Printf("exported: %s\n", fpath)

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("%s: writing file: %v", fpath, err)
	}
	return nil
}

type embedFileSystem struct {
	http.FileSystem
	indexes bool
}

// Open open a file based on name
func (e embedFileSystem) Open(name string) (http.File, error) {
	file, err := e.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	// check if indexing is allowed
	s, _ := file.Stat()
	if s.IsDir() && !e.indexes {
		return nil, fmt.Errorf("dir not available")
	}

	return file, err
}

// Exists tests a path exists
func (e embedFileSystem) Exists(prefix string, path string) bool {
	f, err := e.Open(path)
	if err != nil {
		return false
	}

	// check if indexing is allowed
	s, _ := f.Stat()
	if s.IsDir() && !e.indexes {
		return false
	}

	return true
}

// EmbedFolder create FileSystem from a File System and subdirectory. Has ability to disable index dirs.
func EmbedFolder(fsEmbed fs.FS, targetPath string, index bool) http.FileSystem {
	subFS, err := fs.Sub(fsEmbed, targetPath)
	if err != nil {
		panic(err)
	}
	return embedFileSystem{
		FileSystem: http.FS(subFS),
		indexes:    index,
	}
}
