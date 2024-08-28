import {
  NatsInfo,
  PlugNmeetInfo,
  Recorder,
  WebsocketServerInfo,
} from './utils/interfaces';
import { NatsConnection } from '@nats-io/nats-core';
import {
  jetstream,
  JetStreamClient,
  JetStreamManager,
  jetstreamManager,
} from '@nats-io/jetstream';
import { Kvm } from '@nats-io/kv';
import { connect } from '@nats-io/transport-node/lib/connect';
import * as process from 'node:process';

import { logger } from './utils/helper';
import { addRecorder, sendPing } from './utils/natsTasks';
import {
  PlugNmeetToRecorder,
  PlugNmeetToRecorderSchema,
} from './proto/plugnmeet_recorder_pb';
import { fromJsonString } from '@bufbuild/protobuf';

export default class PNMRecorder {
  private readonly _natsInfo: NatsInfo;
  private readonly _recorder: Recorder;
  private readonly _plugNmeetInfo: PlugNmeetInfo;
  private readonly _websocketServerInfo: WebsocketServerInfo;
  private _nc: NatsConnection | undefined;
  private _js: JetStreamClient | undefined;
  private _jsm: JetStreamManager | undefined;
  private _kvm: Kvm | undefined;

  constructor(config: any) {
    this._natsInfo = config.nats_info;
    this._recorder = config.recorder;
    this._plugNmeetInfo = config.plugNmeet_info;
    this._websocketServerInfo = config.websocket_server;
  }

  get natsInfo(): NatsInfo {
    return this._natsInfo;
  }

  get recorder(): Recorder {
    return this._recorder;
  }

  get plugNmeetInfo(): PlugNmeetInfo {
    return this._plugNmeetInfo;
  }

  get websocketServerInfo(): WebsocketServerInfo {
    return this._websocketServerInfo;
  }

  get nc(): NatsConnection | undefined {
    return this._nc;
  }

  get js(): JetStreamManager | undefined {
    return this._jsm;
  }

  get kvm(): Kvm | undefined {
    return this._kvm;
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

      m.ack();
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
    }, 3000);
    // start immediately
    await sendPing(this._kvm, this.recorder.id);
  }
}
