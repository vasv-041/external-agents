const presets = {
  hook: '{\n  "session_id": "demo-001",\n  "timestamp": "2026-09-06T12:00:00Z",\n  "prompt": "Create hello.txt"\n}',
  'new-format': '{\n  "event": "user_prompt",\n  "session_id": "btw-track3-demo-001",\n  "text": "Add coupon validation to checkout."\n}',
  unknown: '{\n  "event": "future_event_v2",\n  "session_id": "demo-001",\n  "payload": { "new_field": true }\n}',
  chunk: 'Entire keeps transcript bytes intact.\n'
};

const operation = document.querySelector('#operation');
const payload = document.querySelector('#payload');
const output = document.querySelector('#output');
const outputState = document.querySelector('#output-state');
const outputCaption = document.querySelector('#output-caption');
let runCount = 0;

operation.addEventListener('change', () => {
  payload.value = presets[operation.value];
  outputState.textContent = 'ready';
  outputCaption.textContent = 'Input changed - ready to run';
});
document.querySelector('#reset').addEventListener('click', () => { payload.value = presets[operation.value]; run(); });
document.querySelector('#run').addEventListener('click', run);

function pretty(value) { return JSON.stringify(value, null, 2); }
function run() {
  const kind = operation.value;
  runCount += 1;
  outputState.textContent = 'processing';
  let result;
  try {
    if (kind === 'chunk') {
      const bytes = new TextEncoder().encode(payload.value);
      const chunks = [];
      for (let i = 0; i < bytes.length; i += 12) chunks.push(toBase64(bytes.slice(i, i + 12)));
      result = { chunks, chunk_count: chunks.length };
      outputCaption.textContent = `Run #${runCount}: transcript split into safe byte chunks`;
    } else {
      const input = JSON.parse(payload.value);
      if (kind === 'unknown') {
        result = null;
        outputCaption.textContent = `Run #${runCount}: ignored safely - no checkpoint event emitted`;
      } else {
        const isNew = kind === 'new-format';
        result = { type: 2, event: 'TurnStart', session_id: input.session_id, prompt: input.prompt || input.text, source_format: isNew ? 'new-jsonl' : 'qwen-hook', checkpoint_safe: true };
        outputCaption.textContent = `Run #${runCount}: normalized into an Entire TurnStart event`;
      }
    }
    output.textContent = kind === 'chunk' ? pretty(result) : (result === null ? 'null\n\n// unknown event ignored' : pretty(result));
    outputState.textContent = 'success';
  } catch (error) {
    output.textContent = pretty({ error: 'Invalid input', detail: error.message });
    outputCaption.textContent = `Run #${runCount}: input needs valid JSON`;
    outputState.textContent = 'error';
  }
}
function toBase64(bytes) { let binary = ''; bytes.forEach(byte => { binary += String.fromCharCode(byte); }); return btoa(binary); }
payload.value = presets.hook;
run();
