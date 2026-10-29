import json, os, pathlib, time
root=pathlib.Path(os.environ['BENCH_OUTPUT']);stage=os.environ['BENCH_STAGE'];p=root/f'stage-{stage}-ui.json'
(root/'awaiting-ui.json').write_text(json.dumps({'stage':stage,'ready':True}))
end=time.monotonic()+240
while time.monotonic()<end:
 if p.exists():print(p.read_text());break
 time.sleep(.5)
else:print(json.dumps({'unavailable':'UI measurement was not completed within 240 seconds'}))
(root/'awaiting-ui.json').unlink(missing_ok=True)
