import { NatsConnection } from '@nats-io/nats-core';
import { Kvm } from '@nats-io/kv';
import { connect } from '@nats-io/transport-node/lib/connect';
import { create, fromJsonString, toJsonString } from '@bufbuild/protobuf';
import { ChildProcess } from 'concurrently/dist/src/command';
import { fork } from 'child_process';
import {
  FromChildToParentSchema,
  FromParentToChildSchema,
  PlugNmeetToRecorder,
  PlugNmeetToRecorderSchema,
  RecorderServiceType,
  RecorderToPlugNmeetSchema,
  RecordingTasks,
  StartRecorderChildArgsSchema,
} from 'plugnmeet-protocol-js';

import {
  ChildProcessInfoMap,
  NatsInfo,
  PlugNmeetInfo,
  Recorder,
  WebsocketServerInfo,
} from './utils/interfaces';
import { logger, notify } from './utils/helper';
import {
  addRecorder,
  sendPing,
  updateRecorderProgress,
} from './utils/natsTasks';

const PING_INTERVAL = 3 * 1000;

export default class PNMRecorder {
  private readonly _natsInfo: NatsInfo;
  private readonly _recorder: Recorder;
  private readonly _plugNmeetInfo: PlugNmeetInfo;
  private readonly _websocketServerInfo: WebsocketServerInfo;

  private _nc: NatsConnection | undefined;
  private _kvm: Kvm | undefined;

  private readonly _childProcessesInfoMapByChildPid = new Map<
    number,
    ChildProcessInfoMap
  >();
  private readonly _childProcessesMapByRoomSid = new Map<string, any>();

  constructor(config: any) {
    this._natsInfo = config.nats_info;
    this._recorder = config.recorder;
    this._plugNmeetInfo = config.plugNmeet_info;
    this._websocketServerInfo = config.websocket_server;
  }

  get recorder(): Recorder {
    return this._recorder;
  }

  get plugNmeetInfo(): PlugNmeetInfo {
    return this._plugNmeetInfo;
  }

  public get childProcessesInfoMapByChildPid(): Map<
    number,
    ChildProcessInfoMap
  > {
    return this._childProcessesInfoMapByChildPid;
  }

  public get childProcessesMapByRoomSid(): Map<string, any> {
    return this._childProcessesMapByRoomSid;
  }

  public openNatsConnection = async () => {
    try {
      this._nc = await connect({
        servers: this._natsInfo.nats_urls,
        user: this._natsInfo.user,
        pass: this._natsInfo.password,
      });
      logger.info(`connected to ${this._nc?.getServer()}`);

      this._kvm = new Kvm(this._nc);
      //subscriber for PNM events
      this.subscriberToRecorderChannel().then();

      const keyName =
        this._natsInfo.recorder.recorder_info_kv + '-' + this._recorder.id;
      // add this record
      await addRecorder(
        this._kvm,
        this._natsInfo.num_replicas,
        keyName,
        this.recorder.max_limit,
      );
      // start ping
      await this.startPing(keyName);
    } catch (_err) {
      logger.error(`error connecting to ${JSON.stringify(_err)}`);
      process.exit(1);
    }
  };

  private async subscriberToRecorderChannel() {
    if (!this._nc || this._nc.isClosed()) {
      return;
    }
    const sub = this._nc.subscribe(this._natsInfo.recorder.recorder_channel);

    for await (const m of sub) {
      let payload: PlugNmeetToRecorder;
      try {
        payload = fromJsonString(PlugNmeetToRecorderSchema, m.string());
      } catch (e) {
        logger.error(e);
        return;
      }
      if (payload.from !== 'plugnmeet') {
        return;
      }
      logger.info('Main: ' + payload.task.toString());

      switch (payload.task) {
        case RecordingTasks.START_RECORDING:
        case RecordingTasks.START_RTMP:
          if (payload.recorderId === this._recorder.id) {
            this.handleStartRequest(payload);
            m.respond('success');
          }
          break;
        case RecordingTasks.STOP_RECORDING:
        case RecordingTasks.STOP_RTMP:
        case RecordingTasks.STOP:
          // stop the task do not need to verify recorder id
          this.handleStopProcess(BigInt(payload.roomTableId));
          m.respond('success');
          break;
        default:
          logger.error('invalid task');
      }
    }
  }

  private handleStartRequest(payload: PlugNmeetToRecorder) {
    const websocket_url = `${this._websocketServerInfo.host}:${this._websocketServerInfo.port}?auth_token=${this._websocketServerInfo.auth_token}&room_table_id=${payload.roomTableId}&room_id=${payload.roomId}&room_sid=${payload.roomSid}&recording_id=${payload.recordingId}`;

    const sendToChild = create(StartRecorderChildArgsSchema, {
      serviceType: RecorderServiceType.RECORDING,
      roomTableId: payload.roomTableId,
      recordingId: payload.recordingId,
      accessToken: payload.accessToken,
      pnmHost: this._plugNmeetInfo.host,
      pnmJoinHost: this._plugNmeetInfo.join_host,
      websocketUrl: websocket_url,
      width: this._recorder.width,
      height: this._recorder.height,
      xvfbDpi: this._recorder.xvfb_dpi,
      customChromePath: this._recorder.custom_chrome_path,
    });

    if (payload.task === RecordingTasks.START_RECORDING) {
      sendToChild.websocketUrl =
        sendToChild.websocketUrl + '&service=recording';
    } else if (payload.task === RecordingTasks.START_RTMP) {
      sendToChild.websocketUrl =
        sendToChild.websocketUrl + '&service=rtmp&rtmp_url=' + payload.rtmpUrl;
      sendToChild.serviceType = RecorderServiceType.RTMP;
    }

    let child: ChildProcess;

    if (typeof process.env.TS_NODE_DEV !== 'undefined') {
      child = fork(
        'src/recorder',
        [toJsonString(StartRecorderChildArgsSchema, sendToChild)],
        {
          execArgv: ['-r', 'ts-node/register'],
        },
      );
    } else {
      child = fork('dist/recorder', [
        toJsonString(StartRecorderChildArgsSchema, sendToChild),
      ]);
    }

    if (child.pid) {
      const childProcessInfo: ChildProcessInfoMap = {
        serviceType: sendToChild.serviceType,
        recording_id: sendToChild.recordingId,
        room_table_id: BigInt(sendToChild.roomTableId),
      };
      this._childProcessesInfoMapByChildPid.set(child.pid, childProcessInfo);
      this._childProcessesMapByRoomSid.set(
        sendToChild.serviceType + ':' + payload.roomTableId,
        child,
      );
    }

    child.on('message', async (msg: string) => {
      if (child.pid) {
        await this.handleMsgFromChild(msg, child.pid);
      }
    });

    child.on('exit', async (code: number) => {
      if (child.pid) {
        const recordInfo = this._childProcessesInfoMapByChildPid.get(child.pid);

        if (typeof recordInfo !== 'undefined') {
          // we can use same as FromChildToParent message format.
          const fromChild = create(FromChildToParentSchema, {
            msg: code === 0 ? 'no error' : 'had error',
            status: code === 0,
            task:
              recordInfo.serviceType === RecorderServiceType.RECORDING
                ? RecordingTasks.END_RECORDING
                : RecordingTasks.END_RTMP,
            recordingId: recordInfo.recording_id,
            roomTableId: String(recordInfo.room_table_id),
          });
          await this.handleMsgFromChild(
            toJsonString(FromChildToParentSchema, fromChild),
            child.pid,
          );
        }
      }
    });
  }

  private handleStopProcess(roomTableId: bigint) {
    let serviceType: RecorderServiceType | undefined = undefined;
    let task: RecordingTasks | undefined = undefined;
    if (
      this._childProcessesMapByRoomSid.has(
        RecorderServiceType.RTMP + ':' + roomTableId,
      )
    ) {
      serviceType = RecorderServiceType.RTMP;
      task = RecordingTasks.STOP_RTMP;
    } else if (
      this._childProcessesMapByRoomSid.has(
        RecorderServiceType.RECORDING + ':' + roomTableId,
      )
    ) {
      serviceType = RecorderServiceType.RECORDING;
      task = RecordingTasks.STOP_RECORDING;
    }

    if (typeof serviceType === 'undefined' || typeof task === 'undefined') {
      return;
    }

    const child = this._childProcessesMapByRoomSid.get(
      serviceType + ':' + roomTableId,
    );
    if (child) {
      const recordInfo = this._childProcessesInfoMapByChildPid.get(child.pid);
      if (recordInfo) {
        const toChild = create(FromParentToChildSchema, {
          task: task,
          recordingId: recordInfo.recording_id,
          roomTableId: String(recordInfo.room_table_id),
        });
        child.send(toJsonString(FromParentToChildSchema, toChild));
      }
    }
  }

  private async handleMsgFromChild(m: string, pid: number) {
    const msg = fromJsonString(FromChildToParentSchema, m);
    let increment = true;

    const payload = create(RecorderToPlugNmeetSchema, {
      from: 'recorder',
      status: msg.status,
      task: msg.task,
      msg: msg.msg,
      recordingId: msg.recordingId,
      roomTableId: msg.roomTableId,
      recorderId: this._recorder.id, // this recorder ID
    });

    if (
      msg.task === RecordingTasks.END_RECORDING ||
      msg.task === RecordingTasks.END_RTMP
    ) {
      increment = false;
      let serviceType = RecorderServiceType.RECORDING;
      if (payload.task === RecordingTasks.END_RTMP) {
        serviceType = RecorderServiceType.RTMP;
      }
      // clean up
      this._childProcessesInfoMapByChildPid.delete(pid);
      this._childProcessesMapByRoomSid.delete(
        serviceType + ':' + payload.roomSid,
      );
    }

    if (payload) {
      await notify(this._plugNmeetInfo, payload);
      if (this._kvm) {
        const keyName =
          this._natsInfo.recorder.recorder_info_kv + '-' + this._recorder.id;
        await updateRecorderProgress(this._kvm, keyName, increment);
      }
    }
  }

  private async startPing(keyName: string) {
    if (!this._kvm) {
      return;
    }
    setInterval(async () => {
      if (this._kvm) {
        await sendPing(this._kvm, this._natsInfo.num_replicas, keyName);
      }
    }, PING_INTERVAL);
    // start immediately
    await sendPing(this._kvm, this._natsInfo.num_replicas, keyName);
  }
}
