package tui_test

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// The mockClient methods for the HALO routes: tags by channel, tag rename
// and delete, channel tags and files.

func (m *mockClient) GetTags(_ context.Context, channelID string) ([]*model.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []*model.Tag
	for _, t := range m.allTags {
		if t.UsableIn(channelID) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockClient) RenameTag(_ context.Context, tagID, name string) (*model.Tag, error) {
	m.renamedTags = append(m.renamedTags, tagID+"="+name)
	if m.err != nil {
		return nil, m.err
	}
	for _, t := range m.allTags {
		if t.ID == tagID {
			renamed := *t
			renamed.Name = name
			return &renamed, nil
		}
	}
	return &model.Tag{ID: tagID, Name: name}, nil
}

func (m *mockClient) DeleteTag(_ context.Context, tagID string) error {
	m.deletedTags = append(m.deletedTags, tagID)
	return m.err
}

func (m *mockClient) GetChannelTags(_ context.Context, channelID string) ([]*model.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.channelTags[channelID], nil
}

func (m *mockClient) AddChannelTag(_ context.Context, channelID, tagID string) error {
	m.chanTagCalls = append(m.chanTagCalls, "+"+channelID+"#"+tagID)
	return m.err
}

func (m *mockClient) RemoveChannelTag(_ context.Context, channelID, tagID string) error {
	m.chanTagCalls = append(m.chanTagCalls, "-"+channelID+"#"+tagID)
	return m.err
}

func (m *mockClient) FindChannelsByTags(_ context.Context, _ []string, _ string) ([]*model.TaggedChannel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.foundChannels, nil
}

func (m *mockClient) GetFileInfo(_ context.Context, fileID string) (*model.File, error) {
	m.fileInfoCalls = append(m.fileInfoCalls, fileID)
	if m.err != nil {
		return nil, m.err
	}
	if f, ok := m.files[fileID]; ok {
		return f, nil
	}
	return nil, &api.APIError{StatusCode: 404}
}

func (m *mockClient) UploadFile(_ context.Context, channelID, name, contentType, altText string, r io.Reader) (*model.File, error) {
	m.uploads = append(m.uploads, fmt.Sprintf("%s/%s/%s/%s", channelID, name, contentType, altText))
	if m.err != nil {
		return nil, m.err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("f%d", len(m.uploads))
	f := &model.File{ID: id, ChannelID: channelID, Name: name, ContentType: contentType, AltText: altText, SizeBytes: int64(len(data))}
	if m.files == nil {
		m.files = make(map[string]*model.File)
		m.fileBytes = make(map[string][]byte)
	}
	m.files[id] = f
	m.fileBytes[id] = data
	return f, nil
}

func (m *mockClient) DownloadFile(_ context.Context, fileID string, w io.Writer, limit int64) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	data, ok := m.fileBytes[fileID]
	if !ok {
		return 0, &api.APIError{StatusCode: 404}
	}
	if int64(len(data)) > limit {
		return 0, api.ErrFileTooLarge
	}
	return io.Copy(w, bytes.NewReader(data))
}
