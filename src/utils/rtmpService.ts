import { spawn } from 'child_process';
import { logger } from './helper';

export default class RtmpService {
  private ws: any;
  private rtmpUrl: string;

  constructor(ws: any, rtmpUrl: any) {
    this.ws = ws;
    this.rtmpUrl = rtmpUrl;
    this.startService();
  }

  private startService = async () => {
    // prettier-ignore
    const ffmpeg = spawn('ffmpeg', [
      // FFmpeg will read input video from STDIN
      '-i', '-',

      '-c:v', 'libx264',
      '-x264-params', 'keyint=120:scenecut=0',
      '-b:v', '2500k',
      '-video_size', '1280x720',
      '-c:a', 'aac',
      '-b:a', '128k',
      '-ar', '44100',
      '-af', 'highpass=f=200,lowpass=f=2000,afftdn', //https://superuser.com/a/835585
      // '-maxrate', '2000k',
      // '-bufsize', '2000k',
      // '-framerate', '30',
      // '-g', '60',
      '-preset', 'ultrafast',
      '-crf', '5',
      '-vf', 'format=yuv420p',
      '-tune', 'zerolatency',

      // FLV is the container format used in conjunction with RTMP
      '-f', 'flv',
      // The output RTMP URL.
      // For debugging, you could set this to a filename like 'test.flv', and play
      // the resulting file with VLC.
      this.rtmpUrl,
    ]);

    // If FFmpeg stops for any reason, close the WebSocket connection.
    ffmpeg.on('close', (code, signal) => {
      logger.error(
        'FFmpeg child process closed, code ' + code + ', signal ' + signal,
      );
      this.ws.terminate();
    });

    ffmpeg.stdin.on('error', (e) => {
      logger.error('FFmpeg STDIN Error', e);
    });

    //When data comes in from the WebSocket, write it to FFmpeg's STDIN.
    this.ws.on('message', (msg: any) => {
      ffmpeg.stdin.write(msg);
    });

    // If the client disconnects, stop FFmpeg.
    this.ws.on('close', () => {
      ffmpeg.kill('SIGINT');
      this.ws.terminate();
    });
  };
}
