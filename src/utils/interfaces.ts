export interface Recorder {
  id: string;
  max_limit: number;
  post_mp4_convert: boolean;
  custom_chrome_path?: string;
  copy_to_path: CopyToPath;
}

export interface RecorderRedisHashInfo {
  maxLimit: number;
  currentProgress: number;
  lastPing: number;
  created: number;
}

export interface CopyToPath {
  main_path: string;
  sub_path?: string;
}

export interface WebsocketServerInfo {
  port: number;
  host: string;
  auth_token: string;
  ffmpeg_threads: string;
}

export interface PlugNmeetInfo {
  host: string;
  api_key: string;
  api_secret: string;
  join_host?: string;
}

export interface RedisInfo {
  host: string;
  port: number;
  username?: string;
  password?: string;
  db?: number;
  use_tls?: boolean;
  sentinel_master_name?: string;
  sentinel_addresses?: Array<string>;
  sentinel_username?: string;
  sentinel_password?: string;
}

export interface RecorderResp {
  from: string;
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

export interface ChildProcessInfoMap {
  serviceType: number;
  recording_id: string;
  sid: string;
  room_id: string;
}
