import React from "react";
import { AbsoluteFill } from "remotion";
import { TransitionSeries, linearTiming } from "@remotion/transitions";
import { fade } from "@remotion/transitions/fade";
import { slide } from "@remotion/transitions/slide";

import { IntroScene } from "./scenes/IntroScene";
import { ManifestScene } from "./scenes/ManifestScene";
import { ArchitectureScene } from "./scenes/ArchitectureScene";
import { PipelineScene } from "./scenes/PipelineScene";
import { ProvidersScene } from "./scenes/ProvidersScene";
import { WorkflowScene } from "./scenes/WorkflowScene";
import { OutroScene } from "./scenes/OutroScene";

// Each scene duration in frames (at 30fps)
const SCENE_DURATION = 150; // 5 seconds per scene
const TRANSITION_DURATION = 20;

export const ClaudieAnimation: React.FC = () => {
  return (
    <AbsoluteFill>
      <TransitionSeries>
        {/* Scene 1: Intro */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <IntroScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={fade()}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 2: Declarative Manifest */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <ManifestScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={slide({ direction: "from-right" })}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 3: How It Works (flow) */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <WorkflowScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={fade()}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 4: Architecture */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <ArchitectureScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={slide({ direction: "from-left" })}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 5: Pipeline */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <PipelineScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={fade()}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 6: Multi-Cloud Providers */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <ProvidersScene />
        </TransitionSeries.Sequence>

        <TransitionSeries.Transition
          presentation={fade()}
          timing={linearTiming({ durationInFrames: TRANSITION_DURATION })}
        />

        {/* Scene 7: Outro */}
        <TransitionSeries.Sequence durationInFrames={SCENE_DURATION}>
          <OutroScene />
        </TransitionSeries.Sequence>
      </TransitionSeries>
    </AbsoluteFill>
  );
};
