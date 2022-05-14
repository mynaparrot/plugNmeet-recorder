export interface Recorder {
  id: string;
  max_limit: number;
  post_mp4_convert: boolean;
  custom_chrome_path?: string;
  copy_to_path: CopyToPath;
}

export interface CopyToPath {
  main_path: string;
  sub_path?: string;
}

export interface WebsocketServerInfo {
  port: number;
  host: string;
  auth_token: string;
}

export interface PlugNmeetInfo {
  join_host: string;
}

export interface RecorderAddReq {
  from: string;
  task: string;
  recorder_id: string;
  max_limit: number;
}

export interface RecorderPingReq {
  from: string;
  task: string;
  recorder_id: string;
}

export interface RedisInfo {
  host: string;
  port: number;
  username?: string;
  password?: string;
  db?: number;
}

export interface RecorderReq {
  from: string;
  from_server_id: string;
  task: string;
  room_id: string;
  record_id: string;
  sid: string;
  access_token: string;
  recorder_id: string;
  rtmp_url?: string;
}

export interface RecorderArgs {
  from_server_id: string;
  room_id: string;
  record_id: string;
  sid: string;
  access_token: string;
  redisInfo: RedisInfo;
  join_host: string;
  post_mp4_convert: boolean;
  copy_to_path: CopyToPath;
  serviceType: string;
  recorder_id?: string;
  rtmp_url?: string;
  websocket_url: string;
  custom_chrome_path?: string;
}

export interface RecorderResp {
  from: string;
  to_server_id: string;
  task: string;
  status: boolean;
  msg: string;
  record_id: string;
  sid: string;
  room_id: string;
  recorder_id?: string;
  file_path?: string;
  file_size?: number;
}
