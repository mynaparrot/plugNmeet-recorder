import { spawn } from 'child_process';

import { logger } from './helper';
import { FFMPEGOptions } from './interfaces';

export default class RtmpService {
  private ws: any;
  private rtmpUrl: string;
  private readonly ffmpegOptions: FFMPEGOptions;

  constructor(ws: any, ffmpegOptions: FFMPEGOptions, rtmpUrl: any) {
    this.ws = ws;
    this.ffmpegOptions = ffmpegOptions;
    this.rtmpUrl = rtmpUrl;
    this.startService();
  }

  private startService = async () => {
    const options = [];
    if (this.ffmpegOptions.rtmp.pre_input !== '') {
      options.push(...this.ffmpegOptions.rtmp.pre_input.split(' '));
    }
    options.push('-i', '-');
    options.push(...this.ffmpegOptions.rtmp.post_input.split(' '));
    options.push('-f', 'flv', this.rtmpUrl);
    logger.info('ffmpeg options: ' + options);

    const ffmpeg = spawn('ffmpeg', options);

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
