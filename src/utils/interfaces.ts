export interface Recorder {
  id: string;
  max_limit: number;
  post_mp4_convert: boolean;
  custom_chrome_path?: string;
  copy_to_path: CopyToPath;
  width: number;
  height: number;
  xvfb_dpi: number;
  post_processing_scripts?: string[];
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
  host: string;
  api_key: string;
  api_secret: string;
  join_host?: string;
}

export interface NatsInfo {
  nats_urls: string[];
  user: string;
  password: string;
  num_replicas: number;
  recorder: NatsInfoRecorder;
}

export interface NatsInfoRecorder {
  recorder_channel: string;
  recorder_info_kv: string;
}

export interface ChildProcessInfoMap {
  serviceType: number;
  recording_id: string;
  room_table_id: string;
}

export interface FFMPEGOptions {
  recording: {
    pre_input: string;
    post_input: string;
  };
  rtmp: {
    pre_input: string;
    post_input: string;
  };
}

export interface PostProcessScriptData {
  recording_id: string;
  room_table_id: number;
  room_id: string;
  room_sid: string;
  file_path: string; // this will be the full path of the file
  file_size: number;
  recorder_id: string;
}
