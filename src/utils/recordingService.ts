import { spawn } from 'child_process';
import fs from 'fs';

import { PlugNmeetInfo, Recorder, RecorderResp, RedisInfo } from './interfaces';
import { logger, notify } from './helper';

export default class RecordingService {
  private ws: any;
  private recorder: Recorder;
  private plugNmeetInfo: PlugNmeetInfo;
  private redisInfo: RedisInfo;
  private roomId: string;
  private roomSid: string;
  private recordId: string;
  private ffmpegThreads: string;

  constructor(
    ws: any,
    recorder: Recorder,
    plugNmeetInfo: PlugNmeetInfo,
    redisInfo: RedisInfo,
    roomId: any,
    roomSid: any,
    recordId: any,
    ffmpegThreads = '4',
  ) {
    this.ws = ws;
    this.recorder = recorder;
    this.plugNmeetInfo = plugNmeetInfo;
    this.redisInfo = redisInfo;
    this.roomId = roomId;
    this.roomSid = roomSid;
    this.recordId = recordId;
    this.ffmpegThreads = ffmpegThreads;
    this.startService();
  }

  private startService = async () => {
    const copy_to_dir = this.recorder.copy_to_path.main_path;
    let sub_path = '';
    if (this.recorder.copy_to_path.sub_path) {
      // don't forget to add tailing "/"
      sub_path = `${this.recorder.copy_to_path.sub_path}/`;
    }
    const saveToPath = `${copy_to_dir}/${sub_path}${this.roomSid}`;
    if (!fs.existsSync(saveToPath)) {
      await fs.promises.mkdir(saveToPath, {
        recursive: true,
      });
    }

    const file = `${saveToPath}/${this.recordId}.webm`;
    const fileStream = fs.createWriteStream(file, { flags: 'a' });

    this.ws.on('message', (msg: any) => {
      fileStream.write(msg);
    });

    // If the client disconnects.
    this.ws.on('close', async () => {
      fileStream.close();
      this.ws.terminate();

      if (this.recorder.post_mp4_convert) {
        this.convertToMp4(file, saveToPath, sub_path);
      } else {
        const stat = await fs.promises.stat(file);
        const fileSize = (stat.size / (1024 * 1024)).toFixed(2);

        // format: sub_path/roomSid/filename
        const storeFilePath = `${sub_path}${this.roomSid}/${this.recordId}.webm`;
        // now notify to plugNmeet
        this.notifyByRedis(storeFilePath, Number(fileSize));
      }
    });
  };

  private convertToMp4 = async (
    from: string,
    saveToPath: string,
    sub_path: string,
  ) => {
    const mp4File = `${this.recordId}.mp4`;
    const to = `${saveToPath}/${mp4File}`;

    // prettier-ignore
    const ffmpeg = spawn(
      'ffmpeg',
      [
        '-y',
        '-i ', from,
        '-threads', this.ffmpegThreads,
        '-movflags faststart',
        '-c:v copy', // we can copy as Chrome will record in h264 codec
        '-preset veryfast',
        '-vsync vfr',
        to,
      ],
      {
        shell: true,
      },
    );

    ffmpeg.on('close', async (code) => {
      if (code == 0) {
        logger.info('Conversion done to here: ' + to);
        const stat = await fs.promises.stat(to);
        const fileSize = (stat.size / (1024 * 1024)).toFixed(2);

        // format: sub_path/roomSid/filename
        const storeFilePath = `${sub_path}${this.roomSid}/${mp4File}`;
        // now notify to plugNmeet
        await this.notifyByRedis(storeFilePath, Number(fileSize));

        // delete webm file as we don't need it.
        await fs.promises.unlink(from);
      }
    });
  };

  private notifyByRedis = async (filePath: string, file_size: number) => {
    const payload: RecorderResp = {
      from: 'recorder',
      status: true,
      task: 'recording-proceeded',
      msg: 'process completed',
      record_id: this.recordId,
      sid: this.roomSid,
      room_id: this.roomId,
      file_path: filePath,
      file_size: file_size,
      recorder_id: this.recorder.id,
    };

    await notify(this.plugNmeetInfo, payload);
  };
}
