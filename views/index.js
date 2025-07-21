;(() => {
  function dial() {
    const conn = new WebSocket(`ws://${location.host}/subscribe`)

    conn.addEventListener('close', ev => {
      appendLog(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`, true)
      if (ev.code !== 1001) {
        appendLog('Reconnecting in 1s', true)
        setTimeout(dial, 1000)
      }
    })
    conn.addEventListener('open', ev => {
      console.info('websocket connected')
    })

    // This is where we handle messages received.
    conn.addEventListener('message', ev => {
      if (typeof ev.data !== 'string') {
        console.error('unexpected message type', typeof ev.data)
        return
      }
      const p = appendLog(ev.data)
      if (expectingMessage) {
        p.scrollIntoView()
        expectingMessage = false
      }
    })
  }
  // dial()

  const playButton = document.getElementById('play-button')

  // appendLog appends the passed text to messageLog.
  function appendLog(text, error) {
    const p = document.createElement('p')
    // Adding a timestamp to each message makes the log easier to read.
    p.innerText = `${new Date().toLocaleTimeString()}: ${text}`
    if (error) {
      p.style.color = 'red'
      p.style.fontStyle = 'bold'
    }
    messageLog.append(p)
    return p
  }
  appendLog('Submit a message to get started!')

    button.addEventListener('click', ev => {
        dial()
    })
})()
