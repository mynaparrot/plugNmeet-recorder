const args = process.argv.slice(2);

if (!args.length) {
  console.log('no option sent');
  process.exit();
}

// should get JSON value
console.log(args[0]);

/*
 '{"recording_id":"RM_b6UvReYDngee-1700133937567","room_table_id":852,"room_id":"room01","room_sid":"RM_b6UvReYDngee","file_path":"../recording_files/node_01/RM_b6UvReYDngee/RM_b6UvReYDngee-1700133937567.mp4","file_size":0.14,"recorder_id":"node_01"}'
*/

// you should call process.exit() at the end to make the process finish
// otherwise the process may not be clear
process.exit();
