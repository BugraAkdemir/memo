// Generated from measured narration. Every beat has a separate source node.
import { Sequence, useVideoConfig } from "remotion";
import timeline from "../film-timeline.json";
import { Shot } from "./Shot";
export function AuthoredTimeline() {
  const { fps } = useVideoConfig();
  return (
    <>
      <Sequence name="intro" from={0} durationInFrames={539} premountFor={fps}>
        <Shot beat={timeline.beats[0]} />
      </Sequence>
      <Sequence
        name="s01-p0"
        from={539}
        durationInFrames={433}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[1]} />
      </Sequence>
      <Sequence
        name="s01-p1"
        from={972}
        durationInFrames={334}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[2]} />
      </Sequence>
      <Sequence
        name="s02-example"
        from={1306}
        durationInFrames={343}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[3]} />
      </Sequence>
      <Sequence
        name="s02-p0"
        from={1649}
        durationInFrames={423}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[4]} />
      </Sequence>
      <Sequence
        name="s03-p0"
        from={2072}
        durationInFrames={595}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[5]} />
      </Sequence>
      <Sequence
        name="s04-p0"
        from={2667}
        durationInFrames={422}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[6]} />
      </Sequence>
      <Sequence
        name="s04-p1"
        from={3089}
        durationInFrames={86}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[7]} />
      </Sequence>
      <Sequence
        name="s05-p0"
        from={3175}
        durationInFrames={543}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[8]} />
      </Sequence>
      <Sequence
        name="s05-p1"
        from={3718}
        durationInFrames={253}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[9]} />
      </Sequence>
      <Sequence
        name="s06-p0"
        from={3971}
        durationInFrames={394}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[10]} />
      </Sequence>
      <Sequence
        name="s06-p1"
        from={4365}
        durationInFrames={384}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[11]} />
      </Sequence>
      <Sequence
        name="s07-p0"
        from={4749}
        durationInFrames={502}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[12]} />
      </Sequence>
      <Sequence
        name="s07-p1"
        from={5251}
        durationInFrames={93}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[13]} />
      </Sequence>
      <Sequence
        name="s08-example"
        from={5344}
        durationInFrames={556}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[14]} />
      </Sequence>
      <Sequence
        name="s08-p0"
        from={5900}
        durationInFrames={421}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[15]} />
      </Sequence>
      <Sequence
        name="s09-p0"
        from={6321}
        durationInFrames={492}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[16]} />
      </Sequence>
      <Sequence
        name="s10-p0"
        from={6813}
        durationInFrames={513}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[17]} />
      </Sequence>
      <Sequence
        name="s10-p1"
        from={7326}
        durationInFrames={296}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[18]} />
      </Sequence>
      <Sequence
        name="s11-p0"
        from={7622}
        durationInFrames={496}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[19]} />
      </Sequence>
      <Sequence
        name="s11-p1"
        from={8118}
        durationInFrames={106}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[20]} />
      </Sequence>
      <Sequence
        name="s12-p0"
        from={8224}
        durationInFrames={400}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[21]} />
      </Sequence>
      <Sequence
        name="s12-p1"
        from={8624}
        durationInFrames={389}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[22]} />
      </Sequence>
      <Sequence
        name="s12-p2"
        from={9013}
        durationInFrames={73}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[23]} />
      </Sequence>
      <Sequence
        name="s13-p0"
        from={9086}
        durationInFrames={389}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[24]} />
      </Sequence>
      <Sequence
        name="s13-p1"
        from={9475}
        durationInFrames={269}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[25]} />
      </Sequence>
      <Sequence
        name="s14-example"
        from={9744}
        durationInFrames={364}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[26]} />
      </Sequence>
      <Sequence
        name="s14-p0"
        from={10108}
        durationInFrames={377}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[27]} />
      </Sequence>
      <Sequence
        name="s15-p0"
        from={10485}
        durationInFrames={426}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[28]} />
      </Sequence>
      <Sequence
        name="s15-p1"
        from={10911}
        durationInFrames={200}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[29]} />
      </Sequence>
      <Sequence
        name="s16-p0"
        from={11111}
        durationInFrames={450}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[30]} />
      </Sequence>
      <Sequence
        name="s16-p1"
        from={11561}
        durationInFrames={144}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[31]} />
      </Sequence>
      <Sequence
        name="s17-p0"
        from={11705}
        durationInFrames={385}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[32]} />
      </Sequence>
      <Sequence
        name="s17-p1"
        from={12090}
        durationInFrames={441}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[33]} />
      </Sequence>
      <Sequence
        name="s18-example"
        from={12531}
        durationInFrames={440}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[34]} />
      </Sequence>
      <Sequence
        name="s18-p0"
        from={12971}
        durationInFrames={310}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[35]} />
      </Sequence>
      <Sequence
        name="s18-p1"
        from={13281}
        durationInFrames={381}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[36]} />
      </Sequence>
      <Sequence
        name="s19-example"
        from={13662}
        durationInFrames={335}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[37]} />
      </Sequence>
      <Sequence
        name="s19-p0"
        from={13997}
        durationInFrames={487}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[38]} />
      </Sequence>
      <Sequence
        name="s19-p1"
        from={14484}
        durationInFrames={441}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[39]} />
      </Sequence>
      <Sequence
        name="s20-p0"
        from={14925}
        durationInFrames={396}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[40]} />
      </Sequence>
      <Sequence
        name="s20-p1"
        from={15321}
        durationInFrames={196}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[41]} />
      </Sequence>
      <Sequence
        name="s21-p0"
        from={15517}
        durationInFrames={444}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[42]} />
      </Sequence>
      <Sequence
        name="s21-p1"
        from={15961}
        durationInFrames={432}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[43]} />
      </Sequence>
      <Sequence
        name="s21-p2"
        from={16393}
        durationInFrames={235}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[44]} />
      </Sequence>
      <Sequence
        name="s22-p0"
        from={16628}
        durationInFrames={337}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[45]} />
      </Sequence>
      <Sequence
        name="s22-p1"
        from={16965}
        durationInFrames={321}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[46]} />
      </Sequence>
      <Sequence
        name="s22-p2"
        from={17286}
        durationInFrames={178}
        premountFor={fps}
      >
        <Shot beat={timeline.beats[47]} />
      </Sequence>
    </>
  );
}
