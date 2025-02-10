package utils

import (
	"errors"
	"fmt"
	"log"
	"time"

	mtproto "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
)

var senders = make(map[int][]*mtproto.MTProto)

func DlChunk(c *tg.Client, media any, start int, end int) ([]byte, int, int, error) {
	chunkSize := 1024 * 1024
	var buf []byte
	input, dc, size, _, err := tg.GetFileLocation(media)
	if err != nil {
		return nil, 0, 0, err
	}
	if chunkSize > 1048576 || chunkSize%1024 != 0 {
		return nil, 0, 0, errors.New("in precise mode, chunk size must be <= 1 MB and divisible by 1 KB")
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
			return nil, 0, 0, err
		}

		if senders[int(dc)] == nil {
			senders[int(dc)] = make([]*mtproto.MTProto, 0)
		}
		senders[int(dc)] = append(senders[int(dc)], sender)
	}

	startX, stopX := start, end

	for i := start; i < end; i += chunkSize {
		if i+chunkSize > int(size) {
			chunkSize = int(size) - i
		}

		offset := i
		if offset%1024 != 0 {
			offset = i - (i % 1024)
		}

		//offset / (1024 * 1024) == (offset + limit - 1) / (1024 * 1024).
		// this equation should be true for all offsets and limits., check if it is true

		if (offset / (1024 * 1024)) != ((offset + chunkSize - 1) / (1024 * 1024)) {
			fmt.Println("offset:", offset, "chunkSize:", chunkSize)
			fmt.Println("offset / (1024 * 1024):", offset/(1024*1024), "(offset + chunkSize - 1) / (1024 * 1024):", (offset+chunkSize-1)/(1024*1024))
		}

		if offset%1024 != 0 {
			panic("offset not divisible by 1024")
		}

		log.Printf("STREAMER ---> Requesting chunk %d to %d from %d to %d of %d bytes\n", offset, offset+chunkSize, start, end, size)
		startX = offset
		stopX = offset + chunkSize

		part, err := sender.MakeRequest(&tg.UploadGetFileParams{
			Location:     input,
			Limit:        int32(chunkSize),
			Offset:       int64(offset),
			Precise:      true,
			CdnSupported: false,
		})

		chunkSize = 1024 * 1024 // reset chunk size to 1 MB

		if err != nil {
			c.Log.Error(err)
			return []byte{}, 0, 0, nil
		}

		switch v := part.(type) {
		case *tg.UploadFileObj:
			buf = append(buf, v.Bytes...)
		case *tg.UploadFileCdnRedirect:
			return nil, 0, 0, errors.New("cdn redirect not implemented")
		}
	}

	return buf, startX, stopX, nil
}
