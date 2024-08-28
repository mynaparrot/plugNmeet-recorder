import { NatsConnection } from '@nats-io/nats-core';
import {
  jetstream,
  JetStreamClient,
  JetStreamManager,
  jetstreamManager,
} from '@nats-io/jetstream';
import { Kvm } from '@nats-io/kv';
import { connect } from '@nats-io/transport-node/lib/connect';
import { create, fromJsonString, toJsonString } from '@bufbuild/protobuf';
import { ChildProcess } from 'concurrently/dist/src/command';
import { fork } from 'child_process';

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
import {
  FromChildToParentSchema,
  FromParentToChildSchema,
  PlugNmeetToRecorder,
  PlugNmeetToRecorderSchema,
  RecorderServiceType,
  RecorderToPlugNmeetSchema,
  RecordingTasks,
  StartRecorderChildArgsSchema,
} from './proto/plugnmeet_recorder_pb';

const PING_INTERVAL = 3 * 1000;
export default class PNMRecorder {
  private readonly _natsInfo: NatsInfo;
  private readonly _recorder: Recorder;
  private readonly _plugNmeetInfo: PlugNmeetInfo;
  private readonly _websocketServerInfo: WebsocketServerInfo;
  private _nc: NatsConnection | undefined;
  private _js: JetStreamClient | undefined;
  private _jsm: JetStreamManager | undefined;
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

      this._jsm = await jetstreamManager(this._nc);
      this._js = jetstream(this._nc);
      this._kvm = new Kvm(this._nc);

      //subscriber for PNM events
      this.subscriberToSysRecorder();

      // add this record
      await addRecorder(this._kvm, this.recorder.id, this.recorder.max_limit);
      // start ping
      await this.startPing();
    } catch (_err) {
      logger.error(`error connecting to ${JSON.stringify(_err)}`);
      process.exit(1);
    }
  };

  private async subscriberToSysRecorder() {
    if (!this._jsm || !this._js) {
      return;
    }

    await this._jsm.consumers.add(this._natsInfo.subjects.recorder_js_worker, {
      durable_name: this._recorder.id,
      filter_subjects: [
        this._natsInfo.subjects.recorder_js_worker + '.' + this._recorder.id,
      ],
    });

    const consumer = await this._js.consumers.get(
      this._natsInfo.subjects.recorder_js_worker,
      this._recorder.id,
    );

    const sub = await consumer.consume();
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    for await (const m of sub) {
      console.log(m.string());
      let payload: PlugNmeetToRecorder;
      try {
        payload = fromJsonString(PlugNmeetToRecorderSchema, m.string());
      } catch (e) {
        logger.error(e);
        m.ack();
        return;
      }
      if (payload.from !== 'plugnmeet') {
        m.ack();
        return;
      }
      logger.info('Main: ' + payload.task);

      if (
        (payload.task === RecordingTasks.START_RECORDING ||
          payload.task === RecordingTasks.START_RTMP) &&
        payload.recorderId === this._recorder.id
      ) {
        this.handleStartRequest(payload);
      } else if (
        payload.task === RecordingTasks.STOP_RECORDING &&
        this._childProcessesMapByRoomSid.has(
          RecorderServiceType.RECORDING + ':' + payload.roomTableId,
        )
      ) {
        this.handleStopProcess(
          RecordingTasks.STOP_RECORDING,
          RecorderServiceType.RECORDING,
          payload.roomTableId,
        );
      } else if (
        payload.task === RecordingTasks.STOP_RTMP &&
        this._childProcessesMapByRoomSid.has(
          RecorderServiceType.RTMP + ':' + payload.roomTableId,
        )
      ) {
        this.handleStopProcess(
          RecordingTasks.STOP_RTMP,
          RecorderServiceType.RTMP,
          payload.roomTableId,
        );
      } else if (payload.task === RecordingTasks.STOP) {
        // for any stop task when meeting will end or have stop request
        if (
          this._childProcessesMapByRoomSid.has(
            RecorderServiceType.RECORDING + ':' + payload.roomTableId,
          )
        ) {
          this.handleStopProcess(
            RecordingTasks.STOP_RECORDING,
            RecorderServiceType.RECORDING,
            payload.roomTableId,
          );
        }
        if (
          this._childProcessesMapByRoomSid.has(
            RecorderServiceType.RTMP + ':' + payload.roomTableId,
          )
        ) {
          this.handleStopProcess(
            RecordingTasks.STOP_RTMP,
            RecorderServiceType.RTMP,
            payload.roomTableId,
          );
        }
      }

      m.ack();
    }
  }

  private handleStartRequest(payload: PlugNmeetToRecorder) {
    const websocket_url = `${this._websocketServerInfo.host}:${this._websocketServerInfo.port}?auth_token=${this._websocketServerInfo.auth_token}&room_table_id=${payload.roomTableId}&room_id=${payload.roomId}&room_sid=${payload.roomSid}&recording_id=${payload.recordingId}`;

    const toSend = create(StartRecorderChildArgsSchema, {
      roomTableId: payload.roomTableId,
      recordingId: payload.recordingId,
      accessToken: payload.accessToken,
      plugNMeetInfo: {
        host: this._plugNmeetInfo.host,
        apiKey: this._plugNmeetInfo.api_key,
        apiSecret: this._plugNmeetInfo.api_secret,
        joinHost: this._plugNmeetInfo.join_host,
      },
      postMp4Convert: this._recorder.post_mp4_convert,
      copyToPath: {
        mainPath: this._recorder.copy_to_path.main_path,
        subPath: this._recorder.copy_to_path.sub_path,
      },
      recorderId: this._recorder.id,
      serviceType: RecorderServiceType.RECORDING,
      websocketUrl: websocket_url,
    });

    if (this._recorder.custom_chrome_path) {
      toSend.customChromePath = this._recorder.custom_chrome_path;
    }

    if (payload.task === RecordingTasks.START_RECORDING) {
      toSend.websocketUrl = toSend.websocketUrl + '&service=recording';
    } else if (payload.task === RecordingTasks.START_RTMP) {
      toSend.websocketUrl =
        toSend.websocketUrl + '&service=rtmp&rtmp_url=' + payload.rtmpUrl;
      toSend.serviceType = RecorderServiceType.RTMP;
    }

    let child: ChildProcess;

    if (typeof process.env.TS_NODE_DEV !== 'undefined') {
      child = fork(
        'src/recorder',
        [toJsonString(StartRecorderChildArgsSchema, toSend)],
        {
          execArgv: ['-r', 'ts-node/register'],
        },
      );
    } else {
      child = fork('dist/recorder', [
        toJsonString(StartRecorderChildArgsSchema, toSend),
      ]);
    }

    if (child.pid) {
      const childProcessInfo: ChildProcessInfoMap = {
        serviceType: toSend.serviceType,
        recording_id: toSend.recordingId,
        room_table_id: toSend.roomTableId,
      };
      this._childProcessesInfoMapByChildPid.set(child.pid, childProcessInfo);
      this._childProcessesMapByRoomSid.set(
        toSend.serviceType + ':' + payload.roomTableId,
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
          const toChild = create(FromChildToParentSchema, {
            msg: code === 0 ? 'no error' : 'had error',
            status: code === 0,
            task:
              recordInfo.serviceType === RecorderServiceType.RECORDING
                ? RecordingTasks.END_RECORDING
                : RecordingTasks.END_RTMP,
            recordingId: recordInfo.recording_id,
            roomTableId: recordInfo.room_table_id,
          });
          await this.handleMsgFromChild(
            toJsonString(FromChildToParentSchema, toChild),
            child.pid,
          );
        }
      }
    });
  }

  private handleStopProcess(
    task: RecordingTasks,
    serviceType: RecorderServiceType,
    roomTableId: bigint,
  ) {
    const child = this._childProcessesMapByRoomSid.get(
      serviceType + ':' + roomTableId,
    );
    if (child) {
      const recordInfo = this._childProcessesInfoMapByChildPid.get(child.pid);
      if (recordInfo) {
        const toChild = create(FromParentToChildSchema, {
          task: task,
          recordingId: recordInfo.recording_id,
          roomTableId: recordInfo.room_table_id,
        });
        child?.send(toJsonString(FromParentToChildSchema, toChild));
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
        await updateRecorderProgress(this._kvm, this._recorder.id, increment);
      }
    }
  }

  private async startPing() {
    if (!this._kvm) {
      return;
    }
    setInterval(async () => {
      if (this._kvm) {
        await sendPing(this._kvm, this.recorder.id);
      }
    }, PING_INTERVAL);
    // start immediately
    await sendPing(this._kvm, this.recorder.id);
  }
}
