import { Server as WebSocketServer } from 'ws';
import http from 'http';
import yaml from 'js-yaml';
import fs from 'fs';

import {
  FFMPEGOptions,
  PlugNmeetInfo,
  Recorder,
  WebsocketServerInfo,
} from './utils/interfaces';
import RecordingService from './services/recordingService';
import RtmpService from './services/rtmpService';
import { getDefaultFFMPEGOptions, logger } from './utils/helper';

let websocketServerInfo: WebsocketServerInfo,
  recorder: Recorder,
  plugNmeetInfo: PlugNmeetInfo,
  ffmpegOptions: FFMPEGOptions = getDefaultFFMPEGOptions();
try {
  const config: any = yaml.load(fs.readFileSync('config.yaml', 'utf8'));
  websocketServerInfo = config.websocket_server;
  plugNmeetInfo = config.plugNmeet_info;
  recorder = config.recorder;
  if (typeof config.ffmpeg_options !== 'undefined') {
    ffmpegOptions = config.ffmpeg_options;
  }
} catch (e) {
  console.log('Error: ', e);
  process.exit();
}

const killProcess = async () => {
  logger.info('websocketServer: got SIGINT, cleaning up');
  // we can wait before closing everything
  //await sleep(10 * 1000); // 10 seconds
  process.exit();
};
process.on('SIGTERM', killProcess);
process.on('SIGINT', killProcess);

const server = http.createServer().listen(websocketServerInfo.port, () => {
  logger.info('websocket listening port: ' + websocketServerInfo.port);
});

const wss = new WebSocketServer({
  server: server,
});

wss.on('connection', function connection(ws, req) {
  if (!req.url) {
    return;
  }

  const params = new URLSearchParams(req.url.replace('/?', ''));
  const auth_token = params.get('auth_token');
  const service = params.get('service');
  const room_table_id = params.get('room_table_id');
  const room_id = params.get('room_id');
  const room_sid = params.get('room_sid');
  const recording_id = params.get('recording_id');

  if (auth_token !== websocketServerInfo.auth_token || !service) {
    ws.terminate();
    return;
  }

  logger.info(`new ${service} task for ${room_sid}`);

  if (service === 'recording') {
    new RecordingService(
      ws,
      recorder,
      plugNmeetInfo,
      ffmpegOptions,
      BigInt(room_table_id ?? 1),
      room_id ?? '',
      room_sid,
      recording_id,
    );
  } else if (service === 'rtmp') {
    const rtmpUrl = params.get('rtmp_url');
    new RtmpService(ws, ffmpegOptions, rtmpUrl);
  }
});
