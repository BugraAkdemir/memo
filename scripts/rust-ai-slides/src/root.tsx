import { Composition } from "remotion";
import { Movie, ShotPreview } from "./film/Movie";
import timeline from "./film-timeline.json";
export const Root = () => (
  <>
    <Composition
      id="PythonRust"
      component={Movie}
      durationInFrames={timeline.totalFrames}
      width={1920}
      height={1080}
      fps={30}
    />
    <Composition
      id="FilmKaresi"
      component={ShotPreview}
      durationInFrames={180}
      width={1920}
      height={1080}
      fps={30}
      defaultProps={{ beatIndex: 0 }}
    />
  </>
);
