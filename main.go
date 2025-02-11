package main

import (
	"errors"
	"log"
	"main/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mtproto "github.com/amarnathcjd/gogram"
	tg "github.com/amarnathcjd/gogram/telegram"
	_ "github.com/joho/godotenv/autoload"
)

const maxFullDownloadSize = 50 * 1024 * 1024 // 50MB

var client *tg.Client
var senders = make(map[int][]*mtproto.MTProto)

func main() {
	client, _ = tg.NewClient(tg.ClientConfig{
		AppID:    6,
		AppHash:  "eb06d4abfb49dc3eeb1aeb98ae0f581e",
		LogLevel: tg.LogInfo,
	})

	client.Conn()
	client.LoginBot(os.Getenv("BOT_TOKEN"))

	client.AddMessageHandler("/fid", utils.GetBotFileID)
	http.HandleFunc("/stream/", streamHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3010"
	}

	log.Printf("Server running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	fileID := parts[2]

	fi, err := utils.ResolveBotFileID(fileID)
	if err != nil {
		http.Error(w, `{"error": "Invalid file ID"}`, http.StatusBadRequest)
		return
	}

	fileSize := int(fi.(*tg.MessageMediaDocument).Document.(*tg.DocumentObj).Size)
	if fileSize > maxFullDownloadSize {
		http.Error(w, `{"error": "File is larger than 50MB, cannot be streamed"}`, http.StatusForbidden)
		return
	}

	data, err := DlFullFile(client, fi)
	if err != nil {
		http.Error(w, `{"error": "Error fetching full file"}`, http.StatusInternalServerError)
		return
	}

	// w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Type", "video/x-matroska")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func DlFullFile(c *tg.Client, media any) ([]byte, error) {
	chunkSize := 1024 * 1024 // 1MB
	var buf []byte

	input, dc, size, _, err := tg.GetFileLocation(media)
	if err != nil {
		return nil, err
	}

	sender := getSender(c, int(dc))
	if sender == nil {
		return nil, errors.New("failed to get sender")
	}

	log.Printf("STREAMER ---> Downloading full file in chunks (size: %d bytes)\n", size)

	for offset := 0; offset < int(size); offset += chunkSize {
		if offset+chunkSize > int(size) {
			chunkSize = int(size) - offset
			if chunkSize%1024 != 0 {
				chunkSize += 1024 - (chunkSize % 1024)
			}
		}

		if offset%1024 != 0 {
			offset -= offset % 1024
		}

		log.Printf("STREAMER ---> Requesting full file chunk: %d-%d\n", offset, offset+chunkSize)

		part, err := sender.MakeRequest(&tg.UploadGetFileParams{
			Location:     input,
			Limit:        int32(chunkSize),
			Offset:       int64(offset),
			Precise:      true,
			CdnSupported: false,
		})

		if err != nil {
			log.Printf("STREAMER ---> Full file chunk request failed: %s\n", err)
			return nil, err
		}

		switch v := part.(type) {
		case *tg.UploadFileObj:
			buf = append(buf, v.Bytes...)
		case *tg.UploadFileCdnRedirect:
			return nil, errors.New("cdn redirect not implemented")
		}
	}

	return buf, nil
}

func getSender(c *tg.Client, dc int) *mtproto.MTProto {
	if len(senders[dc]) > 0 && senders[dc][0] != nil {
		return senders[dc][0]
	}

	sender, err := c.CreateExportedSender(dc, false)
	if err != nil {
		log.Printf("Failed to create sender for DC %d: %v", dc, err)
		return nil
	}

	senders[dc] = append(senders[dc], sender)

	go func() {
		for {
			time.Sleep(30 * time.Second)
			sender.Ping()
		}
	}()

	return sender
}
