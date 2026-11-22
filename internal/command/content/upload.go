package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/command"
)

// uploadRoute is where files are uploaded, a multipart form whose file is
// its "file" field.
const uploadRoute = "/api/dashboard/files"

// uploadCommand uploads files, each as a multipart form streamed from its
// file, and writes the uuid each was given.
type uploadCommand struct {
	cli   *command.Cli
	group string

	flags *console.FlagSet
}

var _ console.Command = &uploadCommand{}

func newUploadCommand(cli *command.Cli, group string) *uploadCommand {
	return &uploadCommand{cli: cli, group: group}
}

func (c *uploadCommand) Name() string {
	return "upload"
}

func (c *uploadCommand) Description() string {
	return "Upload one or more files"
}

func (c *uploadCommand) Usage() string {
	return c.group + " upload PATH [PATH...]"
}

func (c *uploadCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
}

func (c *uploadCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *uploadCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var errs []error

	for _, path := range c.flags.Args() {
		uuid, err := upload(ctx, client, path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))

			continue
		}

		if _, err := fmt.Fprintln(c.cli.Out, uuid); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}

// upload uploads the file at path and is the uuid it was given.
func upload(ctx context.Context, client *api.Client, path string) (string, error) {
	form, err := newUploadForm(path)
	if err != nil {
		return "", err
	}

	body, err := form.open()
	if err != nil {
		return "", err
	}

	defer func() { _ = body.Close() }()

	request, err := client.NewRequest(ctx, http.MethodPost, uploadRoute, nil, body, form.contentType)
	if err != nil {
		return "", err
	}

	request.ContentLength = form.length
	request.GetBody = form.open

	response, err := client.Send(request)
	if err != nil {
		return "", err
	}

	defer func() { _ = response.Body.Close() }()

	var uploaded struct {
		UUID string `json:"uuid"`
	}

	if err := json.NewDecoder(response.Body).Decode(&uploaded); err != nil {
		return "", fmt.Errorf("reading the answer: %w", err)
	}

	return uploaded.UUID, nil
}

// uploadForm is a multipart form of one file, read from its file each time
// it is sent, so that it is sent again, as it was, once a token is renewed.
type uploadForm struct {
	path        string
	head        []byte
	tail        []byte
	contentType string
	length      int64
}

// newUploadForm is the form of the file at path, named after its base name.
func newUploadForm(path string) (*uploadForm, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}

	var head bytes.Buffer

	writer := multipart.NewWriter(&head)
	if _, err := writer.CreateFormFile("file", filepath.Base(path)); err != nil {
		return nil, err
	}

	tail := []byte("\r\n--" + writer.Boundary() + "--\r\n")

	return &uploadForm{
		path:        path,
		head:        head.Bytes(),
		tail:        tail,
		contentType: writer.FormDataContentType(),
		length:      int64(head.Len()) + info.Size() + int64(len(tail)),
	}, nil
}

// open is the form, from its first byte, its file opened afresh.
func (f *uploadForm) open() (io.ReadCloser, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return nil, err
	}

	return struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(f.head), file, bytes.NewReader(f.tail)), file}, nil
}
