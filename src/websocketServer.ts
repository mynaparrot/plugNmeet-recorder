import { Server as WebSocketServer } from 'ws';
import http from 'http';
import yaml from 'js-yaml';
import fs from 'fs';
import { Recorder, RedisInfo, WebsocketServerInfo } from './utils/interfaces';
import RecordingService from './utils/recordingService';
import RtmpService from './utils/rtmpService';
import { logger } from './utils/helper';

let websocketServerInfo: WebsocketServerInfo;
let recorder: Recorder;
let redisInfo: RedisInfo;
try {
  const config: any = yaml.load(fs.readFileSync('config.yaml', 'utf8'));
  websocketServerInfo = config.websocket_server;
  redisInfo = config.redis_info;
  recorder = config.recorder;
} catch (e) {
  console.log('Error: ', e);
  process.exit();
}

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
  const room_id = params.get('room_id');
  const room_sid = params.get('room_sid');
  const record_id = params.get('record_id');
  const from_server_id = params.get('from_server_id');

  if (auth_token !== websocketServerInfo.auth_token || !service) {
    ws.terminate();
    return;
  }

  logger.info(`new ${service} task for ${room_sid}`);

  if (service === 'recording') {
    new RecordingService(
      ws,
      recorder,
      redisInfo,
      room_id,
      room_sid,
      record_id,
      from_server_id,
    );
  } else if (service === 'rtmp') {
    const rtmpUrl = params.get('rtmp_url');
    new RtmpService(ws, rtmpUrl);
  }
});
