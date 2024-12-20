// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package logger

// Used for storing the log messages in memory.
// Useful for verifying the log messages in unit tests.

import (
	"sync"

	"github.com/sirupsen/logrus"
)

type MemoryLogHook struct {
	subHooksLock sync.Mutex
	subHooks     []*MemoryLogSubHook
}

type MemoryLogSubHook struct {
	parent       *MemoryLogHook
	messagesLock sync.Mutex
	messages     []MemoryLogMessage
}

type MemoryLogMessage struct {
	Message string
	Level   logrus.Level
}

func NewMemoryLogHook() *MemoryLogHook {
	return &MemoryLogHook{}
}

func (h *MemoryLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *MemoryLogHook) Fire(entry *logrus.Entry) error {
	subhooks := h.getSubHooks()
	for _, subhook := range subhooks {
		subhook.addEntry(entry)
	}

	return nil
}

func (h *MemoryLogHook) AddSubHook() *MemoryLogSubHook {
	subhook := &MemoryLogSubHook{
		parent: h,
	}

	h.subHooksLock.Lock()
	defer h.subHooksLock.Unlock()

	newSubHooks := append([]*MemoryLogSubHook(nil), h.subHooks...)
	newSubHooks = append(newSubHooks, subhook)
	h.subHooks = newSubHooks

	return subhook
}

func (h *MemoryLogHook) RemoveSubHook(subHook *MemoryLogSubHook) {
	h.subHooksLock.Lock()
	defer h.subHooksLock.Unlock()

	newSubHooks := []*MemoryLogSubHook(nil)
	for _, entry := range h.subHooks {
		if entry == subHook {
			continue
		}

		newSubHooks = append(newSubHooks, entry)
	}

	h.subHooks = newSubHooks
}

func (h *MemoryLogHook) getSubHooks() []*MemoryLogSubHook {
	h.subHooksLock.Lock()
	defer h.subHooksLock.Unlock()
	return h.subHooks
}

func (h *MemoryLogSubHook) addEntry(entry *logrus.Entry) {
	message := MemoryLogMessage{
		Message: entry.Message,
		Level:   entry.Level,
	}

	h.messagesLock.Lock()
	defer h.messagesLock.Unlock()
	h.messages = append(h.messages, message)
}

func (h *MemoryLogSubHook) Close() {
	h.parent.RemoveSubHook(h)
}

func (h *MemoryLogSubHook) ConsumeMessages() []MemoryLogMessage {
	h.messagesLock.Lock()
	defer h.messagesLock.Unlock()

	messages := h.messages
	h.messages = nil
	return messages
}
