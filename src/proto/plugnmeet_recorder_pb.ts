// @generated by protoc-gen-es v0.0.10 with parameter "target=ts"
// @generated from file plugnmeet_recorder.proto (package plugnmeet, syntax proto3)
/* eslint-disable */
/* @ts-nocheck */

import type {BinaryReadOptions, FieldList, JsonReadOptions, JsonValue, PartialMessage, PlainMessage} from "@bufbuild/protobuf";
import {Message, proto3} from "@bufbuild/protobuf";

/**
 * @generated from enum plugnmeet.RecordingTasks
 */
export enum RecordingTasks {
  /**
   * @generated from enum value: START_RECORDING = 0;
   */
  START_RECORDING = 0,

  /**
   * @generated from enum value: STOP_RECORDING = 1;
   */
  STOP_RECORDING = 1,

  /**
   * @generated from enum value: START_RTMP = 2;
   */
  START_RTMP = 2,

  /**
   * @generated from enum value: STOP_RTMP = 3;
   */
  STOP_RTMP = 3,

  /**
   * @generated from enum value: END_RECORDING = 4;
   */
  END_RECORDING = 4,

  /**
   * @generated from enum value: END_RTMP = 5;
   */
  END_RTMP = 5,

  /**
   * @generated from enum value: RECORDING_PROCEEDED = 6;
   */
  RECORDING_PROCEEDED = 6,
}
// Retrieve enum metadata with: proto3.getEnumType(RecordingTasks)
proto3.util.setEnumType(RecordingTasks, "plugnmeet.RecordingTasks", [
  { no: 0, name: "START_RECORDING" },
  { no: 1, name: "STOP_RECORDING" },
  { no: 2, name: "START_RTMP" },
  { no: 3, name: "STOP_RTMP" },
  { no: 4, name: "END_RECORDING" },
  { no: 5, name: "END_RTMP" },
  { no: 6, name: "RECORDING_PROCEEDED" },
]);

/**
 * @generated from enum plugnmeet.RecorderServiceType
 */
export enum RecorderServiceType {
  /**
   * @generated from enum value: RECORDING = 0;
   */
  RECORDING = 0,

  /**
   * @generated from enum value: RTMP = 1;
   */
  RTMP = 1,
}
// Retrieve enum metadata with: proto3.getEnumType(RecorderServiceType)
proto3.util.setEnumType(RecorderServiceType, "plugnmeet.RecorderServiceType", [
  { no: 0, name: "RECORDING" },
  { no: 1, name: "RTMP" },
]);

/**
 * @generated from message plugnmeet.PlugNmeetToRecorder
 */
export class PlugNmeetToRecorder extends Message<PlugNmeetToRecorder> {
  /**
   * @generated from field: string from = 1;
   */
  from = "";

  /**
   * @generated from field: plugnmeet.RecordingTasks task = 2;
   */
  task = RecordingTasks.START_RECORDING;

  /**
   * @generated from field: string room_id = 3;
   */
  roomId = "";

  /**
   * @generated from field: string room_sid = 4;
   */
  roomSid = "";

  /**
   * @generated from field: string recording_id = 5;
   */
  recordingId = "";

  /**
   * @generated from field: string recorder_id = 6;
   */
  recorderId = "";

  /**
   * @generated from field: string access_token = 7;
   */
  accessToken = "";

  /**
   * @generated from field: optional string rtmp_url = 8;
   */
  rtmpUrl?: string;

  constructor(data?: PartialMessage<PlugNmeetToRecorder>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.PlugNmeetToRecorder";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "from", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 2, name: "task", kind: "enum", T: proto3.getEnumType(RecordingTasks) },
    { no: 3, name: "room_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 4, name: "room_sid", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 5, name: "recording_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 6, name: "recorder_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 7, name: "access_token", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 8, name: "rtmp_url", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): PlugNmeetToRecorder {
    return new PlugNmeetToRecorder().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): PlugNmeetToRecorder {
    return new PlugNmeetToRecorder().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): PlugNmeetToRecorder {
    return new PlugNmeetToRecorder().fromJsonString(jsonString, options);
  }

  static equals(a: PlugNmeetToRecorder | PlainMessage<PlugNmeetToRecorder> | undefined, b: PlugNmeetToRecorder | PlainMessage<PlugNmeetToRecorder> | undefined): boolean {
    return proto3.util.equals(PlugNmeetToRecorder, a, b);
  }
}

/**
 * @generated from message plugnmeet.RecorderToPlugNmeet
 */
export class RecorderToPlugNmeet extends Message<RecorderToPlugNmeet> {
  /**
   * @generated from field: string from = 1;
   */
  from = "";

  /**
   * @generated from field: plugnmeet.RecordingTasks task = 2;
   */
  task = RecordingTasks.START_RECORDING;

  /**
   * @generated from field: bool status = 3;
   */
  status = false;

  /**
   * @generated from field: string msg = 4;
   */
  msg = "";

  /**
   * @generated from field: string recording_id = 5;
   */
  recordingId = "";

  /**
   * @generated from field: string room_id = 6;
   */
  roomId = "";

  /**
   * @generated from field: string room_sid = 7;
   */
  roomSid = "";

  /**
   * @generated from field: string recorder_id = 8;
   */
  recorderId = "";

  /**
   * @generated from field: string file_path = 9;
   */
  filePath = "";

  /**
   * @generated from field: float file_size = 10;
   */
  fileSize = 0;

  constructor(data?: PartialMessage<RecorderToPlugNmeet>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.RecorderToPlugNmeet";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "from", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 2, name: "task", kind: "enum", T: proto3.getEnumType(RecordingTasks) },
    { no: 3, name: "status", kind: "scalar", T: 8 /* ScalarType.BOOL */ },
    { no: 4, name: "msg", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 5, name: "recording_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 6, name: "room_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 7, name: "room_sid", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 8, name: "recorder_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 9, name: "file_path", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 10, name: "file_size", kind: "scalar", T: 2 /* ScalarType.FLOAT */ },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): RecorderToPlugNmeet {
    return new RecorderToPlugNmeet().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): RecorderToPlugNmeet {
    return new RecorderToPlugNmeet().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): RecorderToPlugNmeet {
    return new RecorderToPlugNmeet().fromJsonString(jsonString, options);
  }

  static equals(a: RecorderToPlugNmeet | PlainMessage<RecorderToPlugNmeet> | undefined, b: RecorderToPlugNmeet | PlainMessage<RecorderToPlugNmeet> | undefined): boolean {
    return proto3.util.equals(RecorderToPlugNmeet, a, b);
  }
}

/**
 * @generated from message plugnmeet.FromParentToChild
 */
export class FromParentToChild extends Message<FromParentToChild> {
  /**
   * @generated from field: plugnmeet.RecordingTasks task = 1;
   */
  task = RecordingTasks.START_RECORDING;

  /**
   * @generated from field: string recording_id = 2;
   */
  recordingId = "";

  /**
   * @generated from field: string room_id = 3;
   */
  roomId = "";

  /**
   * @generated from field: string room_sid = 4;
   */
  roomSid = "";

  constructor(data?: PartialMessage<FromParentToChild>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.FromParentToChild";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "task", kind: "enum", T: proto3.getEnumType(RecordingTasks) },
    { no: 2, name: "recording_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 3, name: "room_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 4, name: "room_sid", kind: "scalar", T: 9 /* ScalarType.STRING */ },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): FromParentToChild {
    return new FromParentToChild().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): FromParentToChild {
    return new FromParentToChild().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): FromParentToChild {
    return new FromParentToChild().fromJsonString(jsonString, options);
  }

  static equals(a: FromParentToChild | PlainMessage<FromParentToChild> | undefined, b: FromParentToChild | PlainMessage<FromParentToChild> | undefined): boolean {
    return proto3.util.equals(FromParentToChild, a, b);
  }
}

/**
 * @generated from message plugnmeet.FromChildToParent
 */
export class FromChildToParent extends Message<FromChildToParent> {
  /**
   * @generated from field: plugnmeet.RecordingTasks task = 1;
   */
  task = RecordingTasks.START_RECORDING;

  /**
   * @generated from field: bool status = 2;
   */
  status = false;

  /**
   * @generated from field: string msg = 3;
   */
  msg = "";

  /**
   * @generated from field: string recording_id = 4;
   */
  recordingId = "";

  /**
   * @generated from field: string room_id = 5;
   */
  roomId = "";

  /**
   * @generated from field: string room_sid = 6;
   */
  roomSid = "";

  constructor(data?: PartialMessage<FromChildToParent>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.FromChildToParent";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "task", kind: "enum", T: proto3.getEnumType(RecordingTasks) },
    { no: 2, name: "status", kind: "scalar", T: 8 /* ScalarType.BOOL */ },
    { no: 3, name: "msg", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 4, name: "recording_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 5, name: "room_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 6, name: "room_sid", kind: "scalar", T: 9 /* ScalarType.STRING */ },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): FromChildToParent {
    return new FromChildToParent().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): FromChildToParent {
    return new FromChildToParent().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): FromChildToParent {
    return new FromChildToParent().fromJsonString(jsonString, options);
  }

  static equals(a: FromChildToParent | PlainMessage<FromChildToParent> | undefined, b: FromChildToParent | PlainMessage<FromChildToParent> | undefined): boolean {
    return proto3.util.equals(FromChildToParent, a, b);
  }
}

/**
 * @generated from message plugnmeet.StartRecorderChildArgs
 */
export class StartRecorderChildArgs extends Message<StartRecorderChildArgs> {
  /**
   * @generated from field: string room_id = 1;
   */
  roomId = "";

  /**
   * @generated from field: string recording_id = 2;
   */
  recordingId = "";

  /**
   * @generated from field: string room_sid = 3;
   */
  roomSid = "";

  /**
   * @generated from field: string access_token = 4;
   */
  accessToken = "";

  /**
   * @generated from field: plugnmeet.PlugNmeetInfo plug_n_meet_info = 5;
   */
  plugNMeetInfo?: PlugNmeetInfo;

  /**
   * @generated from field: bool post_mp4_convert = 6;
   */
  postMp4Convert = false;

  /**
   * @generated from field: plugnmeet.CopyToPath copy_to_path = 7;
   */
  copyToPath?: CopyToPath;

  /**
   * @generated from field: plugnmeet.RecorderServiceType serviceType = 8;
   */
  serviceType = RecorderServiceType.RECORDING;

  /**
   * @generated from field: optional string recorder_id = 9;
   */
  recorderId?: string;

  /**
   * @generated from field: optional string rtmp_url = 10;
   */
  rtmpUrl?: string;

  /**
   * @generated from field: string websocket_url = 11;
   */
  websocketUrl = "";

  /**
   * @generated from field: optional string custom_chrome_path = 12;
   */
  customChromePath?: string;

  constructor(data?: PartialMessage<StartRecorderChildArgs>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.StartRecorderChildArgs";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "room_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 2, name: "recording_id", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 3, name: "room_sid", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 4, name: "access_token", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 5, name: "plug_n_meet_info", kind: "message", T: PlugNmeetInfo },
    { no: 6, name: "post_mp4_convert", kind: "scalar", T: 8 /* ScalarType.BOOL */ },
    { no: 7, name: "copy_to_path", kind: "message", T: CopyToPath },
    { no: 8, name: "serviceType", kind: "enum", T: proto3.getEnumType(RecorderServiceType) },
    { no: 9, name: "recorder_id", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
    { no: 10, name: "rtmp_url", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
    { no: 11, name: "websocket_url", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 12, name: "custom_chrome_path", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): StartRecorderChildArgs {
    return new StartRecorderChildArgs().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): StartRecorderChildArgs {
    return new StartRecorderChildArgs().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): StartRecorderChildArgs {
    return new StartRecorderChildArgs().fromJsonString(jsonString, options);
  }

  static equals(a: StartRecorderChildArgs | PlainMessage<StartRecorderChildArgs> | undefined, b: StartRecorderChildArgs | PlainMessage<StartRecorderChildArgs> | undefined): boolean {
    return proto3.util.equals(StartRecorderChildArgs, a, b);
  }
}

/**
 * @generated from message plugnmeet.PlugNmeetInfo
 */
export class PlugNmeetInfo extends Message<PlugNmeetInfo> {
  /**
   * @generated from field: string host = 1;
   */
  host = "";

  /**
   * @generated from field: string api_key = 2;
   */
  apiKey = "";

  /**
   * @generated from field: string api_secret = 3;
   */
  apiSecret = "";

  /**
   * @generated from field: optional string join_host = 4;
   */
  joinHost?: string;

  constructor(data?: PartialMessage<PlugNmeetInfo>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.PlugNmeetInfo";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "host", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 2, name: "api_key", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 3, name: "api_secret", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 4, name: "join_host", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): PlugNmeetInfo {
    return new PlugNmeetInfo().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): PlugNmeetInfo {
    return new PlugNmeetInfo().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): PlugNmeetInfo {
    return new PlugNmeetInfo().fromJsonString(jsonString, options);
  }

  static equals(a: PlugNmeetInfo | PlainMessage<PlugNmeetInfo> | undefined, b: PlugNmeetInfo | PlainMessage<PlugNmeetInfo> | undefined): boolean {
    return proto3.util.equals(PlugNmeetInfo, a, b);
  }
}

/**
 * @generated from message plugnmeet.CopyToPath
 */
export class CopyToPath extends Message<CopyToPath> {
  /**
   * @generated from field: string main_path = 1;
   */
  mainPath = "";

  /**
   * @generated from field: optional string sub_path = 2;
   */
  subPath?: string;

  constructor(data?: PartialMessage<CopyToPath>) {
    super();
    proto3.util.initPartial(data, this);
  }

  static readonly runtime = proto3;
  static readonly typeName = "plugnmeet.CopyToPath";
  static readonly fields: FieldList = proto3.util.newFieldList(() => [
    { no: 1, name: "main_path", kind: "scalar", T: 9 /* ScalarType.STRING */ },
    { no: 2, name: "sub_path", kind: "scalar", T: 9 /* ScalarType.STRING */, opt: true },
  ]);

  static fromBinary(bytes: Uint8Array, options?: Partial<BinaryReadOptions>): CopyToPath {
    return new CopyToPath().fromBinary(bytes, options);
  }

  static fromJson(jsonValue: JsonValue, options?: Partial<JsonReadOptions>): CopyToPath {
    return new CopyToPath().fromJson(jsonValue, options);
  }

  static fromJsonString(jsonString: string, options?: Partial<JsonReadOptions>): CopyToPath {
    return new CopyToPath().fromJsonString(jsonString, options);
  }

  static equals(a: CopyToPath | PlainMessage<CopyToPath> | undefined, b: CopyToPath | PlainMessage<CopyToPath> | undefined): boolean {
    return proto3.util.equals(CopyToPath, a, b);
  }
}

