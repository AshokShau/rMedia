package utils

import (
	"encoding/base64"
	"encoding/binary"
	"errors"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func PackBotFileID(file any) string {
	var (
		fID, accessHash int64
		fileType, dcID  int32
		fileSize        int64
	)

switchFileType:
	switch f := file.(type) {
	case *tg.DocumentObj:
		fID = f.ID
		accessHash = f.AccessHash
		fileType = 5
		dcID = f.DcID
		fileSize = f.Size
		for _, attr := range f.Attributes {
			switch attr := attr.(type) {
			case *tg.DocumentAttributeAudio:
				if attr.Voice {
					fileType = 3
					break switchFileType
				}
				fileType = 9
				break switchFileType
			case *tg.DocumentAttributeVideo:
				if attr.RoundMessage {
					fileType = 13
					break switchFileType
				}
				fileType = 4
				break switchFileType
			}
		}
	case *tg.MessageMediaDocument:
		file = f.Document
		goto switchFileType
	default:
		return ""
	}

	if fID == 0 || accessHash == 0 || fileType == 0 || dcID == 0 {
		return ""
	}

	buf := make([]byte, 4+8+8+8)
	binary.LittleEndian.PutUint32(buf[0:], uint32(fileType)|uint32(dcID)<<24)
	binary.LittleEndian.PutUint64(buf[4:], uint64(fID))
	binary.LittleEndian.PutUint64(buf[4+8:], uint64(accessHash))
	binary.LittleEndian.PutUint64(buf[4+8+8:], uint64(fileSize))
	return base64.RawURLEncoding.EncodeToString(buf)
}

func unpack(fileID string) (int64, int64, int32, int32, int64) {
	data, err := base64.RawURLEncoding.DecodeString(fileID)
	if err != nil {
		return 0, 0, 0, 0, 0
	}

	if len(data) == 4+8+8+8 {
		tmp := binary.LittleEndian.Uint32(data[0:])
		fileType := int32(tmp & 0x00FFFFFF)
		dcID := int32((tmp >> 24) & 0xFF)
		fID := int64(binary.LittleEndian.Uint64(data[4:]))
		accessHash := int64(binary.LittleEndian.Uint64(data[4+8:]))
		fileSize := int64(binary.LittleEndian.Uint64(data[4+8+8:]))
		return fID, accessHash, fileType, dcID, fileSize
	}

	return 0, 0, 0, 0, 0
}

func ResolveBotFileID(fileId string) (tg.MessageMedia, error) {
	fID, accessHash, fileType, dcID, fileSize := unpack(fileId)
	if fID == 0 || accessHash == 0 || fileType == 0 || dcID == 0 {
		return nil, errors.New("failed to resolve file id: unrecognized format")
	}
	switch fileType {
	case 2:
	case 3, 4, 5, 9, 13:
		var attributes = []tg.DocumentAttribute{}
		switch fileType {
		case 3:
			attributes = append(attributes, &tg.DocumentAttributeAudio{
				Voice: true,
			})
		case 4:
			attributes = append(attributes, &tg.DocumentAttributeVideo{
				RoundMessage: false,
			})
		case 9:
			attributes = append(attributes, &tg.DocumentAttributeAudio{})
		case 13:
			attributes = append(attributes, &tg.DocumentAttributeVideo{
				RoundMessage: true,
			})
		}
		return &tg.MessageMediaDocument{
			Document: &tg.DocumentObj{
				ID:         fID,
				AccessHash: accessHash,
				DcID:       dcID,
				Attributes: attributes,
				Size:       fileSize,
			},
		}, nil
	}
	return nil, errors.New("failed to resolve file id: unknown file type")
}

func GetBotFileID(m *tg.NewMessage) error {
	if !m.IsReply() {
		m.Reply("Reply to a file to get its file ID")
		return nil
	}

	r, err := m.GetReplyMessage()
	if err != nil {
		return err
	}

	if r.Document() == nil {
		m.Reply("Only document files are supported")
		return nil
	}

	fileID := PackBotFileID(r.Document())
	if fileID == "" {
		m.Reply("Failed to get file ID")
		return nil
	}

	m.Reply("File ID: <code>" + fileID + "</code>")
	return nil
}
