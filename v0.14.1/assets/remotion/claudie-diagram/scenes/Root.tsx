import { Composition, Still } from "remotion";
import { ClaudieAnimation } from "./ClaudieAnimation";
import { ClusterDiagram, ClusterDiagramProps } from "./ClusterDiagram";
import {
  ClusterDiagramAnimation,
  ClusterDiagramAnimationProps,
} from "./ClusterDiagramAnimation";

// 7 scenes x 150 frames = 1050, minus 6 transitions x 20 = 120
// Total: 1050 - 120 = 930 frames = 31 seconds at 30fps
const TOTAL_DURATION = 930;

const DIAGRAM_W = 1250;
const DIAGRAM_H = 540;

export const RemotionRoot: React.FC = () => {
  return (
    <>
      <Composition
        id="ClaudieAnimation"
        component={ClaudieAnimation}
        durationInFrames={TOTAL_DURATION}
        fps={30}
        width={1080}
        height={620}
      />
      <Composition
        id="ClusterDiagramAnimation"
        component={ClusterDiagramAnimation}
        durationInFrames={450}
        fps={30}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "transparent" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Composition
        id="ClusterDiagramAnimation-light"
        component={ClusterDiagramAnimation}
        durationInFrames={450}
        fps={30}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "light" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Composition
        id="ClusterDiagramAnimation-dark"
        component={ClusterDiagramAnimation}
        durationInFrames={450}
        fps={30}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "dark" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Composition
        id="ClusterDiagramAnimation-transparent-dark"
        component={ClusterDiagramAnimation}
        durationInFrames={450}
        fps={30}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "transparent-dark" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Still
        id="ClusterDiagramV2-transparent-dark"
        component={ClusterDiagramAnimation}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "transparent-dark" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Still
        id="ClusterDiagramV2-transparent"
        component={ClusterDiagramAnimation}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "transparent" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Still
        id="ClusterDiagramV2-light"
        component={ClusterDiagramAnimation}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "light" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Still
        id="ClusterDiagramV2-dark"
        component={ClusterDiagramAnimation}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={
          { theme: "dark" } satisfies ClusterDiagramAnimationProps
        }
      />
      <Still
        id="ClusterDiagram-transparent"
        component={ClusterDiagram}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={{ theme: "transparent" } satisfies ClusterDiagramProps}
      />
      <Still
        id="ClusterDiagram-light"
        component={ClusterDiagram}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={{ theme: "light" } satisfies ClusterDiagramProps}
      />
      <Still
        id="ClusterDiagram-dark"
        component={ClusterDiagram}
        width={DIAGRAM_W}
        height={DIAGRAM_H}
        defaultProps={{ theme: "dark" } satisfies ClusterDiagramProps}
      />
    </>
  );
};
