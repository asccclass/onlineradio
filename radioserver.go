package main

import (
   "bytes"
   "io"
   "log"
   "net/http"
   "os"
   "sync"
   "time"
   "github.com/gorilla/mux"
)

type ConnectionPool struct {
   bufferChannelMap map[chan []byte]struct{}
   mu               sync.Mutex
}

type SryStreamer struct {
   ConnPool	*ConnectionPool
}

func (cp *ConnectionPool) AddConnection(bufferChannel chan []byte) {
   defer cp.mu.Unlock()
   cp.mu.Lock()
   cp.bufferChannelMap[bufferChannel] = struct{}{}
}

func (cp *ConnectionPool) DeleteConnection(bufferChannel chan []byte) {
   defer cp.mu.Unlock()
   cp.mu.Lock()
   delete(cp.bufferChannelMap, bufferChannel)
}

func (cp *ConnectionPool) Broadcast(buffer []byte) {
   defer cp.mu.Unlock()
   cp.mu.Lock()

   for bufferChannel, _ := range cp.bufferChannelMap {
      clonedBuffer := make([]byte, 4096)
      copy(clonedBuffer, buffer)
      select {
      case bufferChannel <- clonedBuffer:
      default:
      }
   }
}

func(app *SryStreamer) stream(content []byte) {
   buffer := make([]byte, 4096)
   for {
      // clear() is a new builtin function introduced in go 1.21. Just reinitialize the buffer if on a lower version.
      clear(buffer)
      tempfile := bytes.NewReader(content)
      ticker := time.NewTicker(time.Millisecond * 250)

      for range ticker.C {
         _, err := tempfile.Read(buffer)
         if err == io.EOF {
            ticker.Stop()
            break
         }
         app.ConnPool.Broadcast(buffer)
      }
   }
}

// 讀取音樂並撥放
func(app *SryStreamer) LoadMusicFromweb(w http.ResponseWriter, r *http.Request) {
   params := mux.Vars(r)

   fname := "./data/" + params["record"] + /" + params["music"] + ".acc" // ./data/0001.acc
   file, err := os.Open(fname)
   if err != nil {
      fmt.Println(err.Error())
      return
   }
   ctn, err := io.ReadAll(file)
   if err != nil {
      fmt.Println(err.Error())
      return
   }
   go app.stream(ctn)

   w.Header().Add("Content-Type", "audio/aac")
   w.Header().Add("Connection", "keep-alive")
   flusher, ok := w.(http.Flusher)
   if !ok {
      fmt.Println("Could not create flusher")
      return
   }
   bufferChannel := make(chan []byte)
   app.ConnPool.AddConnection(bufferChannel)
   fmt.Printf("%s has connected\n", r.Host)
   for {
      buf := <-bufferChannel
      if _, err := w.Write(buf); err != nil {
         app.ConnPool.DeleteConnection(bufferChannel)
         log.Printf("%s's connection has been closed\n", r.Host)
         return
      }
      flusher.Flush()
   }
}

// Web Router
func(app *SryStreamer) AddRouter(router *mux.Router) {
   router.HandleFunc("/{record}/{music}", app.LoadMusicFromWeb)
}

func NewStreamer()(*SryStreamer, error) {
   bufferChannelMap := make(map[chan []byte]struct{})
   connPool := &ConnectionPool{bufferChannelMap: bufferChannelMap}
   return &SryStreamer {
      ConnPool: connPool,
   }, nil
}
