package utils

import (
	"errors"
	"fmt"
	"time"

	mtproto "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
)

var senders = make(map[int][]*mtproto.MTProto)

const CHUNK_SIZE = 1024 * 1024

func GetFileChunks(c *tg.Client, file tg.MessageMedia, offset, limit int) ([]byte, error) {
	chunk, _, err := downloadChunk(c, file, offset, offset+limit, CHUNK_SIZE)
	return chunk, err
}

func downloadChunk(c *tg.Client, media tg.MessageMedia, start, end, chunkSize int) ([]byte, string, error) {
	var buf []byte
	input, dc, size, name, err := tg.GetFileLocation(media)
	if err != nil {
		return nil, "", err
	}

	if (1048576 % chunkSize) != 0 {
		fmt.Println("chunk size must be a multiple of 1048576 (1MB)")
		return nil, "", errors.New("chunk size must be a multiple of 1048576 (1MB)")
	}

	if end > int(size) {
		end = int(size)
	}

	// if chunkSize > 1048576 {
	// 	chunkSize = 1048576
	// }

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
			for { // keep connection alive
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

	startAligned := (start / chunkSize) * chunkSize
	for curr := startAligned; curr < end; curr += chunkSize {
		if curr+chunkSize > end {
			chunkSize = end - curr
		}
		part, err := sender.MakeRequest(&tg.UploadGetFileParams{
			Location:     input,
			Limit:        int32(chunkSize),
			Offset:       int64(curr),
			Precise:      true,
			CdnSupported: false,
		})

		if err != nil {
			c.Log.Error(err)
			continue
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
