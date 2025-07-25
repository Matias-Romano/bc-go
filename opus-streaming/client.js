import { OpusDecoder } from 'opus-decoder';

let audioCtx = null;
let decoder = null;
let nextStartTime = 0;

// Manejador del botón (requiere interacción del usuario)
document.getElementById('play-button').addEventListener('click', () => {
    // Inicializar AudioContext y Decoder
    audioCtx = new AudioContext({ sampleRate: 44100 }); // Ajusta según tu audio
    decoder = new OpusDecoder({
        sampleRate: 44100,
        channels: 2,
        forceStereo: false,
        preSkip: 0,
        streamCount: 1,
        coupledStreamCount: 0,
        channelMappingTable: [0, 1]
    });
    startWebSocket();
});

function startWebSocket() {
    const ws = new WebSocket('ws://localhost:42069/play');

    ws.onopen = () => {
        console.log('WebSocket conectado');
    };

    ws.onmessage = async (event) => {
        // Recibir paquetes como ArrayBuffer
        const packet = await event.data.arrayBuffer();
        const opusPacket = new Uint8Array(packet)

        try {
            // Decodificar el paquete OPUS a PCM (Float32Array)
            const { channelData, samplesDecoded } = decoder.decodeFrame(opusPacket);
            if (samplesDecoded > 0) {
                playAudioBuffer(channelData); // Mono: usar el primer canal
            } else {
                console.warn('No se decodificaron muestras');
                // Opcional: manejar paquetes perdidos con interpolación (necesita lógica adicional)
            }
        } catch (err) {
            console.error('Error al decodificar:', err);
        }
    };

    ws.onclose = () => {
        console.log('WebSocket cerrado');
    };

    ws.onerror = (err) => {
        console.error('Error en WebSocket:', err);
    };
}

// Reproducir un AudioBuffer
function playAudioBuffer(channelData) {
    // Crear un AudioBuffer para los datos PCM
    const buffer = audioCtx.createBuffer(1, channelData[0].length, audioCtx.sampleRate);
    buffer.getChannelData(0).set(channelData[0]);
    buffer.getChannelData(1).set(channelData[1]);

    const source = audioCtx.createBufferSource();
    source.buffer = buffer;
    source.connect(audioCtx.destination);

    // Programar la reproducción para evitar cortes
    const currentTime = audioCtx.currentTime;
    const startTime = Math.max(currentTime, nextStartTime);
    source.start(startTime);
    nextStartTime = startTime + audioData.duration;

    // Opcional: cerrar el AudioContext cuando termine
    source.onended = () => {
        if (audioCtx.state === 'running' && audioBuffer.length === 0) {
            audioCtx.suspend();
        }
    };
}
