package repository

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"fyne.io/fyne"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/storage/repository"
)

// declare conformance with repository types
var _ repository.Repository = (*FileRepository)(nil)
var _ repository.WriteableRepository = (*FileRepository)(nil)
var _ repository.HierarchicalRepository = (*FileRepository)(nil)
var _ repository.ListableRepository = (*FileRepository)(nil)
var _ repository.MovableRepository = (*FileRepository)(nil)
var _ repository.CopyableRepository = (*FileRepository)(nil)

var _ fyne.URIReadCloser = (*file)(nil)
var _ fyne.URIWriteCloser = (*file)(nil)

type file struct {
	*os.File
	uri fyne.URI
}

func (f *file) URI() fyne.URI {
	return f.uri
}

// FileRepository implements a simple wrapper around golang's filesystem
// interface libraries. It should be registered by the driver on platforms
// where it is appropriate to do so.
//
// This repository is suitable to handle file:// schemes.
//
// Since: 2.0.0
type FileRepository struct {
}

// NewFileRepository creates a new FileRepository instance. It must be
// given the scheme it is registered for. The caller needs to call
// repository.Register() on the result of this function.
//
// Since: 2.0.0
func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

// Exists implements repository.Repository.Exists
//
// Since: 2.0.0
func (r *FileRepository) Exists(u fyne.URI) (bool, error) {
	p := u.Path()

	_, err := os.Stat(p)
	ok := false

	if err == nil {
		ok = true
	} else if os.IsNotExist(err) {
		err = nil
	}

	return ok, err
}

func openFile(uri fyne.URI, create bool) (*file, error) {
	if uri.Scheme() != "file" {
		return nil, fmt.Errorf("invalid URI for file: %s", uri)
	}

	path := uri.String()[7:]
	var f *os.File
	var err error
	if create {
		f, err = os.Create(path) // If it exists this will truncate which is what we wanted
	} else {
		f, err = os.Open(path)
	}
	return &file{File: f, uri: uri}, err
}

// Reader implements repository.Repository.Reader
//
// Since: 2.0.0
func (r *FileRepository) Reader(u fyne.URI) (fyne.URIReadCloser, error) {
	return openFile(u, false)
}

// CanRead implements repository.Repository.CanRead
//
// Since: 2.0.0
func (r *FileRepository) CanRead(u fyne.URI) (bool, error) {
	f, err := os.OpenFile(u.Path(), os.O_RDONLY, 0666)
	if err == nil {
		defer f.Close()
	} else {

		if os.IsPermission(err) {
			return false, nil
		}

		if os.IsNotExist(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}
	}

	return true, nil
}

// Destroy implements repository.Repository.Destroy
func (r *FileRepository) Destroy(scheme string) {
	// do nothing
}

// Writer implements repository.WriteableRepository.Writer
//
// Since: 2.0.0
func (r *FileRepository) Writer(u fyne.URI) (fyne.URIWriteCloser, error) {
	return openFile(u, true)
}

// CanWrite implements repository.WriteableRepository.CanWrite
//
// Since: 2.0.0
func (r *FileRepository) CanWrite(u fyne.URI) (bool, error) {
	f, err := os.OpenFile(u.Path(), os.O_WRONLY, 0666)
	if err == nil {
		defer f.Close()
	} else {

		if os.IsPermission(err) {
			return false, nil
		}

		if os.IsNotExist(err) {
			// We may need to do extra logic to check if the
			// directory is writeable, but presumably the
			// IsPermission check covers this.
			return true, nil
		}

		if err != nil {
			return false, err
		}
	}

	return true, nil
}

// Delete implements repository.WriteableRepository.Delete
//
// Since: 2.0.0
func (r *FileRepository) Delete(u fyne.URI) error {
	return os.Remove(u.Path())
}

// Parent implements repository.HierarchicalRepository.Parent
//
// Since: 2.0.0
func (r *FileRepository) Parent(u fyne.URI) (fyne.URI, error) {
	s := u.String()

	// trim trailing slash
	if s[len(s)-1] == '/' {
		s = s[0 : len(s)-1]
	}

	// trim the scheme
	s = s[len(u.Scheme())+3:]

	// Completely empty URI with just a scheme
	if len(s) == 0 {
		return nil, repository.ErrURIRoot
	}

	parent := ""
	if u.Scheme() == "file" {
		// use the system native path resolution
		parent = filepath.Dir(s)
		if parent[len(parent)-1] != filepath.Separator {
			parent += "/"
		}

		// only root is it's own parent
		if filepath.Clean(parent) == filepath.Clean(s) {
			return nil, repository.ErrURIRoot
		}

		return storage.ParseURI(u.Scheme() + "://" + parent)
	}

	// Per the spec for storage.Parent(), we should only need to handle
	// it for schemes we are registered to.
	return nil, fmt.Errorf("FileRepository cannot handle unknown scheme '%s'", u.Scheme())
}

// Child implements repository.HierarchicalRepository.Child
//
// Since: 2.0.0
func (r *FileRepository) Child(u fyne.URI, component string) (fyne.URI, error) {
	newURI := u.Scheme() + "://" + u.Authority()
	newURI += path.Join(u.Path(), component)

	// stick the query and fragment back on the end
	if len(u.Query()) > 0 {
		newURI += "?" + u.Query()
	}
	if len(u.Fragment()) > 0 {
		newURI += "#" + u.Fragment()
	}

	return storage.ParseURI(newURI)
}

// List implements repository.HierarchicalRepository.List()
//
// Since: 2.0.0
func (r *FileRepository) List(u fyne.URI) ([]fyne.URI, error) {
	if u.Scheme() != "file" {
		return nil, fmt.Errorf("unsupported URL protocol")
	}

	path := u.Path()
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	urilist := []fyne.URI{}

	for _, f := range files {
		uri, err := storage.ParseURI("file://" + filepath.Join(path, f.Name()))
		if err != nil {
			return nil, err
		}
		urilist = append(urilist, uri)
	}

	return urilist, nil
}

// CanList implements repository.HierarchicalRepository.CanList()
//
// Since: 2.0.0
func (r *FileRepository) CanList(u fyne.URI) (bool, error) {
	p := u.Path()
	info, err := os.Stat(p)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !info.IsDir() {
		return false, nil
	}

	// We know it is a directory, but we don't know if we can read it, so
	// we'll just try to do so and see if we get a permissions error.
	f, err := os.Open(p)
	if err == nil {
		_, err = f.Readdir(1)
		f.Close()
	}

	if err != nil {
		return false, err
	}

	if os.IsPermission(err) {
		return false, nil
	}

	// it is a directory, and checking the permissions did not error out
	return true, nil
}

// Copy implements repository.CopyableRepository.Copy()
//
// Since: 2.0.0
func (r *FileRepository) Copy(source, destination fyne.URI) error {
	// NOTE: as far as I can tell, golang does not have an optimized Copy
	// function - everything I can find on the 'net suggests doing more
	// or less the equivalent of GenericCopy(), hence why that is used.

	return repository.GenericCopy(source, destination)
}

// Move implements repository.MovableRepository.Move()
//
// Since: 2.0.0
func (r *FileRepository) Move(source, destination fyne.URI) error {
	// NOTE: as far as I can tell, golang does not have an optimized Move
	// function - everything I can find on the 'net suggests doing more
	// or less the equivalent of GenericMove(), hence why that is used.

	return repository.GenericMove(source, destination)
}
