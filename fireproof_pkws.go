package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/mschoch/pkws"
)

type fireproofContext struct {
	svc Service
}

// Fireproof is an implementation of the pkws.Server interface
// that attempts the logic of the Fireproof partykit server
// https://github.com/fireproof-storage/connect/blob/cab6c350129b6b713c2d459ec0991ee010cce910/src/partykit/server.ts
type Fireproof struct {
	log  *slog.Logger
	room *pkws.Room
	//clockHead map[string]*CRDTEntry

	ctx *fireproofContext
}

func NewFireproof(ctx *fireproofContext, room *pkws.Room, log *slog.Logger) pkws.Server {
	return &Fireproof{
		log:  log,
		room: room,
		//clockHead: make(map[string]*CRDTEntry),
		ctx: ctx,
	}
}

func (l *Fireproof) OnStart() {
	l.log.Info("fireproof started")

	//l.ctx.svc.Meta(l.room.Name())
	//
	//headBytes, err := l.room.Storage().Get("main")
	//if err != nil {
	//	l.log.Error("error getting main from storage", "err", err)
	//	return
	//}
	//
	//if headBytes != nil {
	//	var heads map[string]*CRDTEntry
	//	err = json.Unmarshal(headBytes, &heads)
	//	if err != nil {
	//		l.log.Error("error unmarshaling head bytes", "err", err)
	//	}
	//	l.log.Info("set clockhead in start", "clockhead", heads)
	//	l.clockHead = heads
	//}
}

func (l *Fireproof) OnConnect(conn *pkws.Connection) {
	l.log.Info("fireproof connected", "id", conn.ID())
	//
	//for _, v := range l.clockHead {
	//	crdtEntryJSON, err := json.Marshal(v)
	//	if err != nil {
	//		l.log.Error("error marshaling crdt entry", "err", err)
	//		return
	//	}
	//	conn.Send(crdtEntryJSON)
	//}

	metas, err := l.ctx.svc.Meta(l.room.Name(), "main")
	if err != nil {
		l.log.Error("error getting meta", "err", err)
	}

	for _, meta := range metas {
		crdtEntryJSON, err := json.Marshal(meta)
		if err != nil {
			l.log.Error("error marshaling crdt entry", "err", err)
			return
		}
		conn.Send(crdtEntryJSON)
	}
}

func (l *Fireproof) OnMessage(msg []byte, sender *pkws.Connection) {
	l.log.Info("received", "message", string(msg), "from", sender.ID())
	l.onMessageInternal(msg, sender.ID())
}

func (l *Fireproof) onMessageInternal(msg []byte, senderId string) {
	var crdtEntries []*CRDTEntry
	l.log.Info("message internal", "message", string(msg))
	err := json.Unmarshal(msg, &crdtEntries)
	if err != nil {
		l.log.Error("error unmarshaling received msg", "err", err)
		return
	}

	if len(crdtEntries) > 0 {
		cid := crdtEntries[0].Cid
		//parents := crdtEntries[0].Parents
		//l.clockHead[cid] = crdtEntries[0]
		//for _, p := range parents {
		//	delete(l.clockHead, p)
		//}

		var raw []byte
		raw, err = json.Marshal(crdtEntries[0])
		if err != nil {
			l.log.Error("error marshaling first crdt entry back to json", "err", err)
		}

		err = l.ctx.svc.SetMeta(l.room.Name(), "main", &MetaRequest{
			CID:     cid,
			Data:    crdtEntries[0].Data,
			Parents: crdtEntries[0].Parents,
		}, raw)
		if err != nil {
			l.log.Error("error setting meta", "err", err)
		}
	}

	l.room.BroadcastExcept(msg, []string{senderId})
}

func (l *Fireproof) OnClose(conn *pkws.Connection) {
	l.log.Info("fireproof closed", "id", conn.ID())
}

func (l *Fireproof) OnError(conn *pkws.Connection, err error) {
	l.log.Error("fireproof error", "err", err)
}

func (l *Fireproof) OnRequest(w http.ResponseWriter, r *http.Request) {
	l.log.Info("fireproof request", "url", r.URL.String(), "method", r.Method)

	addCORSHeaders(w)

	// CORS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	carId := r.FormValue("car")
	if len(carId) > 0 {
		// data
		switch r.Method {
		case http.MethodPut:
			//carBuf, err := io.ReadAll(r.Body)
			//if err != nil {
			//	l.log.Error("error reading request body", "err", err)
			//	encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error reading request body"})
			//	return
			//}

			err := l.ctx.svc.SetData(l.room.Name(), carId, r.Body)
			//err = l.room.Storage().Put(fmt.Sprintf("car-%s", carId), carBuf)
			if err != nil {
				l.log.Error("error putting to room storage", "err", err)
				encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error putting to room storage"})
				return
			}
			encodeFireproofJSONResponse(w, http.StatusCreated, Res{OK: true})
			return

		case http.MethodGet:
			carReader, err := l.ctx.svc.Data(l.room.Name(), carId)
			//carBuf, err := l.room.Storage().Get(fmt.Sprintf("car-%s", carId))
			if err != nil || carReader == nil {
				encodeFireproofJSONResponse(w, http.StatusNotFound, Res{OK: false})
				return
			}

			carBuf, err := io.ReadAll(carReader)
			if err != nil {
				encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error reading car buf"})
				return
			}
			if len(carBuf) == 0 {
				encodeFireproofJSONResponse(w, http.StatusNotFound, Res{OK: false, Error: "CAR not found"})
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(carBuf)
			return

		case http.MethodDelete:
			// FIXME no delete car

			//existed, err := l.room.Storage().Delete(fmt.Sprintf("car-%s", carId))
			//if err != nil {
			//	l.log.Error("error deleting from room storage", "err", err.Error())
			//	encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error deleting from room storage"})
			//	return
			//}
			//if !existed {
			//	encodeFireproofJSONResponse(w, http.StatusNotFound, Res{OK: false, Error: "CAR not found"})
			//	return
			//}
			encodeFireproofJSONResponse(w, http.StatusCreated, Res{OK: true})
			return
		default:
			l.log.Error("method not allowed", "method", r.Method)
			encodeFireproofJSONResponse(w, http.StatusMethodNotAllowed, Res{OK: false, Error: "Method not allowed"})
			return
		}

	} else {
		// meta
		switch r.Method {
		case http.MethodGet:
			metaVals, err := l.ctx.svc.Meta(l.room.Name(), "main")
			if err != nil {
				encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "get meta oops"})
				return
			}
			//metaVals := make([]*CRDTEntry, 0, len(l.clockHead))
			//for _, v := range l.clockHead {
			//	metaVals = append(metaVals, v)
			//}
			encodeFireproofJSONResponse(w, http.StatusOK, metaVals)
			return
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				l.log.Error("error reading request body", "err", err)
				encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error reading request body"})
				return
			}
			l.onMessageInternal(body, "server")
			encodeFireproofJSONResponse(w, http.StatusOK, Res{OK: true})
			return
		case http.MethodDelete:
			// FIXME no delete meta

			//l.clockHead = make(map[string]*CRDTEntry)
			//clockHeadBytes, err := json.Marshal(l.clockHead)
			//if err != nil {
			//	l.log.Error("error marshaling clockhead", "err", err)
			//	encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error marshaling clockhead"})
			//	return
			//}
			//err = l.room.Storage().Put("main", clockHeadBytes)
			//if err != nil {
			//	l.log.Error("error putting to room storage", "err", err)
			//	encodeFireproofJSONResponse(w, http.StatusInternalServerError, Res{OK: false, Error: "error putting to room storage"})
			//	return
			//}
			encodeFireproofJSONResponse(w, http.StatusOK, Res{OK: true})
			return
		default:
			l.log.Error("method not allowed", "method", r.Method)
			encodeFireproofJSONResponse(w, http.StatusMethodNotAllowed, Res{OK: false, Error: "Method not allowed"})
			return
		}
	}
}

func (l *Fireproof) OnAlarm() {
	l.log.Info("fireproof alarm")
}

type CRDTEntry struct {
	Data    string   `json:"data"`
	Cid     string   `json:"cid"`
	Parents []string `json:"parents"`
}

func addCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers",
		"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func encodeFireproofJSONResponse(w http.ResponseWriter, status int, response any) error {
	var err error
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch t := response.(type) {
	case error:
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": t.Error(),
		})
	default:
		w.WriteHeader(status)
		err = json.NewEncoder(w).Encode(t)
	}

	return err
}

type Res struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty""`
}
