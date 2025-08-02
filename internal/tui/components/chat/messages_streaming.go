package chat

// AppendStreamingChunk appends a chunk to the streaming message
func (m *MessageListModel) AppendStreamingChunk(chunk string) {
	if m.isStreaming {
		m.streamingMsg += chunk
		// The View() method will handle updating the display
	}
}