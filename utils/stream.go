package utils

import (
	"errors"
	"log"
	"time"

	mtproto "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
)

var senders = make(map[int][]*mtproto.MTProto)

const CHUNK_SIZE = 1024 * 1024

func GetFileChunks(c *tg.Client, file tg.MessageMedia, start, end int) ([]byte, error) {
	chunk, _, err := DlChunk(c, file, start, end, CHUNK_SIZE)
	return chunk, err
}

func DlChunk(c *tg.Client, media any, start int, end int, chunkSize int) ([]byte, string, error) {
	var buf []byte
	input, dc, size, name, err := tg.GetFileLocation(media)
	if err != nil {
		return nil, "", err
	}
	if chunkSize > 1048576 || chunkSize%1024 != 0 {
		return nil, "", errors.New("in precise mode, chunk size must be <= 1 MB and divisible by 1 KB")
	}

	if end > int(size) {
		end = int(size)
	}

	var sender *mtproto.MTProto
	for _, s := range senders[int(dc)] {
		if s != nil {
			sender = s
			break
		}
	}

	if sender == nil {
		sender, err = c.CreateExportedSender(int(dc), false)
		go func() {
			for {
				time.Sleep(30 * time.Second)
				sender.Ping()
			}
		}()

		if err != nil {
			return nil, "", err
		}

		if senders[int(dc)] == nil {
			senders[int(dc)] = make([]*mtproto.MTProto, 0)
		}
		senders[int(dc)] = append(senders[int(dc)], sender)
	}

	for i := start; i < end; i += chunkSize {
		if i+chunkSize > int(size) {
			chunkSize = int(size) - i
		}

		offset := i
		if offset%1024 != 0 {
			offset = i - (i % 1024)
		}

		log.Printf("STREAMER ---> Requesting chunk %d to %d from %d to %d of %d bytes\n", offset, offset+chunkSize, start, end, size)

		part, err := sender.MakeRequest(&tg.UploadGetFileParams{
			Location:     input,
			Limit:        int32(chunkSize),
			Offset:       int64(offset),
			Precise:      true,
			CdnSupported: false,
		})

		if err != nil {
			c.Log.Error(err)
			return []byte{}, "", nil
		}

		switch v := part.(type) {
		case *tg.UploadFileObj:
			buf = append(buf, v.Bytes...)
		case *tg.UploadFileCdnRedirect:
			return nil, "", errors.New("cdn redirect not implemented")
		}
	}

	return buf, name, nil
}
