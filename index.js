; (() => {
    function dial() {
        const conn = new WebSocket(`ws://${location.host}/play`)

        conn.addEventListener('close', ev => {
            console.info(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`)
            // if (ev.code !== 1001) {
            //     console.info('Reconnecting in 1s')
            //     setTimeout(dial, 1000)
            // }
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
            console.info('received: ', ev.data)
        })
        conn.addEventListener('error', ev => {
            console.error(ev)
        })
    }
    // dial()

    const playButton = document.getElementById('play-button')

    playButton.addEventListener('click', ev => {
        dial()
    })
})()
