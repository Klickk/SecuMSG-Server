//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/google/uuid"

	"messages/pkg/msgclient"
)

func main() {
	js.Global().Set("msgClientInit", js.FuncOf(initDevice))
	js.Global().Set("msgClientPrepareSend", js.FuncOf(prepareSend))
	js.Global().Set("msgClientHandleEnvelope", js.FuncOf(handleEnvelope))
	js.Global().Set("msgClientStateInfo", js.FuncOf(stateInfo))
	select {}
}

func initDevice(this js.Value, args []js.Value) any {
	return async(func(resolve, reject js.Value) {
		if len(args) == 0 {
			reject.Invoke("missing init options")
			return
		}
		opts := args[0]
		cfg := msgclient.InitOptions{
			KeysBaseURL:     opts.Get("keysURL").String(),
			MessagesBaseURL: opts.Get("messagesURL").String(),
			UserID:          opts.Get("userID").String(),
			DeviceID:        opts.Get("deviceID").String(),
		}
		state, resp, err := msgclient.RegisterDevice(context.Background(), cfg)
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		data, err := state.Marshal()
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		out := map[string]any{
			"state":          string(data),
			"userId":         resp.UserID,
			"deviceId":       resp.DeviceID,
			"keysUrl":        state.KeysBaseURL(),
			"messagesUrl":    state.MessagesBaseURL(),
			"oneTimePrekeys": resp.OneTimePreKeys,
		}
		resolve.Invoke(js.ValueOf(out))
	})
}

func prepareSend(this js.Value, args []js.Value) any {
	return async(func(resolve, reject js.Value) {
		if len(args) < 1 {
			reject.Invoke("missing arguments")
			return
		}
		input := args[0]
		stateStr := input.Get("state").String()
		convIDStr := input.Get("convId").String()
		toIDStr := input.Get("toDeviceId").String()
		plaintext := input.Get("plaintext").String()

		state, err := msgclient.LoadStateFromJSON([]byte(stateStr))
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		convID, err := uuid.Parse(convIDStr)
		if err != nil {
			reject.Invoke(fmt.Sprintf("invalid conversation id: %v", err))
			return
		}
		toID, err := uuid.Parse(toIDStr)
		if err != nil {
			reject.Invoke(fmt.Sprintf("invalid recipient id: %v", err))
			return
		}
		req, err := state.PrepareSend(convID, toID, plaintext)
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		stateJSON, err := state.Marshal()
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		body, err := json.Marshal(req)
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			reject.Invoke(err.Error())
			return
		}
		out := map[string]any{
			"state":   string(stateJSON),
			"request": payload,
		}
		resolve.Invoke(js.ValueOf(out))
	})
}

func handleEnvelope(this js.Value, args []js.Value) any {
	return async(func(resolve, reject js.Value) {
		if len(args) == 0 {
			reject.Invoke("missing arguments")
			return
		}
		input := args[0]
		stateStr := input.Get("state").String()
		envelopeJSON := input.Get("envelope").String()

		state, err := msgclient.LoadStateFromJSON([]byte(stateStr))
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		var env msgclient.InboundEnvelope
		if err := json.Unmarshal([]byte(envelopeJSON), &env); err != nil {
			reject.Invoke(err.Error())
			return
		}
		plaintext, err := state.HandleEnvelope(&env)
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		stateJSON, err := state.Marshal()
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		out := map[string]any{
			"state":     string(stateJSON),
			"plaintext": plaintext,
		}
		resolve.Invoke(js.ValueOf(out))
	})
}

func stateInfo(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	stateStr := args[0].String()
	state, err := msgclient.LoadStateFromJSON([]byte(stateStr))
	if err != nil {
		return js.Null()
	}
	info := map[string]any{
		"userId":      state.UserID(),
		"deviceId":    state.DeviceID(),
		"keysUrl":     state.KeysBaseURL(),
		"messagesUrl": state.MessagesBaseURL(),
	}
	return js.ValueOf(info)
}

func async(fn func(resolve, reject js.Value)) js.Value {
	promise := js.Global().Get("Promise")
	handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]
		go fn(resolve, reject)
		return nil
	})
	return promise.New(handler)
}
