import React from "react";
import {
  AbsoluteFill,
  useCurrentFrame,
  useVideoConfig,
  interpolate,
  spring,
  Sequence,
} from "remotion";
import { loadFont } from "@remotion/google-fonts/Inter";
import { COLORS } from "../colors";

const { fontFamily } = loadFont("normal", {
  weights: ["400", "700"],
  subsets: ["latin"],
});

const FLOW_STEPS = [
  {
    label: "User applies\nInputManifest",
    color: COLORS.primary,
    icon: "👤",
  },
  {
    label: "Operator\nreconciles",
    color: COLORS.accent,
    icon: "🎯",
  },
  {
    label: "Manager\nschedules tasks",
    color: COLORS.warning,
    icon: "🧠",
  },
  {
    label: "Pipeline\nprovisioning",
    color: COLORS.secondary,
    icon: "⚙️",
  },
  {
    label: "Cluster\nready!",
    color: COLORS.secondary,
    icon: "✅",
  },
];

export const WorkflowScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const headerOpacity = spring({ frame, fps, config: { damping: 200 } });

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        padding: 60,
        justifyContent: "flex-start",
      }}
    >
      {/* Header */}
      <div
        style={{
          opacity: headerOpacity,
          display: "flex",
          alignItems: "center",
          gap: 16,
          marginBottom: 50,
        }}
      >
        <div
          style={{
            width: 8,
            height: 40,
            borderRadius: 4,
            background: `linear-gradient(180deg, ${COLORS.warning}, ${COLORS.primary})`,
          }}
        />
        <div style={{ fontSize: 42, fontWeight: 700, color: COLORS.text }}>
          How It Works
        </div>
      </div>

      {/* Flow diagram */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 0,
          marginBottom: 60,
        }}
      >
        {FLOW_STEPS.map((step, i) => {
          const stepDelay = 10 + i * 18;
          const stepProgress = spring({
            frame,
            fps,
            delay: stepDelay,
            config: { damping: 15, stiffness: 120 },
          });
          const stepOpacity = spring({
            frame,
            fps,
            delay: stepDelay,
            config: { damping: 200 },
          });

          const arrowOpacity =
            i < FLOW_STEPS.length - 1
              ? spring({
                  frame,
                  fps,
                  delay: stepDelay + 12,
                  config: { damping: 200 },
                })
              : 0;

          return (
            <Sequence key={step.label} from={0} layout="none">
              <div style={{ display: "flex", alignItems: "center" }}>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    width: 140,
                    opacity: stepOpacity,
                    transform: `scale(${stepProgress})`,
                  }}
                >
                  <div
                    style={{
                      width: 64,
                      height: 64,
                      borderRadius: 20,
                      background: `${step.color}18`,
                      border: `2px solid ${step.color}`,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontSize: 30,
                      marginBottom: 12,
                      boxShadow: `0 0 30px ${step.color}22`,
                    }}
                  >
                    {step.icon}
                  </div>
                  <div
                    style={{
                      fontSize: 14,
                      fontWeight: 700,
                      color: COLORS.text,
                      textAlign: "center",
                      lineHeight: 1.4,
                      whiteSpace: "pre-line",
                    }}
                  >
                    {step.label}
                  </div>
                </div>

                {/* Arrow */}
                {i < FLOW_STEPS.length - 1 && (
                  <div
                    style={{
                      width: 50,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      opacity: arrowOpacity,
                      marginBottom: 30,
                    }}
                  >
                    <svg width="40" height="20" viewBox="0 0 40 20">
                      <line
                        x1="0"
                        y1="10"
                        x2="30"
                        y2="10"
                        stroke={COLORS.border}
                        strokeWidth="2"
                      />
                      <polygon
                        points="28,5 38,10 28,15"
                        fill={COLORS.border}
                      />
                    </svg>
                  </div>
                )}
              </div>
            </Sequence>
          );
        })}
      </div>

      {/* State management explanation */}
      <div
        style={{
          display: "flex",
          gap: 24,
          justifyContent: "center",
        }}
      >
        {[
          {
            title: "Current State",
            desc: "What infrastructure exists right now",
            color: COLORS.textMuted,
            delay: 80,
          },
          {
            title: "→",
            desc: "Reconcile",
            color: COLORS.warning,
            delay: 90,
            isArrow: true,
          },
          {
            title: "Desired State",
            desc: "What the InputManifest declares",
            color: COLORS.secondary,
            delay: 100,
          },
        ].map((item) => {
          const itemOpacity = spring({
            frame,
            fps,
            delay: item.delay,
            config: { damping: 200 },
          });

          if ("isArrow" in item && item.isArrow) {
            return (
              <Sequence key="arrow" from={0} layout="none">
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    opacity: itemOpacity,
                    fontSize: 40,
                    color: item.color,
                    padding: "0 8px",
                  }}
                >
                  →
                </div>
              </Sequence>
            );
          }

          return (
            <Sequence key={item.title} from={0} layout="none">
              <div
                style={{
                  background: COLORS.card,
                  borderRadius: 16,
                  padding: "24px 32px",
                  border: `2px solid ${item.color}44`,
                  opacity: itemOpacity,
                  textAlign: "center",
                  minWidth: 260,
                }}
              >
                <div
                  style={{
                    fontSize: 22,
                    fontWeight: 700,
                    color: item.color,
                    marginBottom: 8,
                  }}
                >
                  {item.title}
                </div>
                <div style={{ fontSize: 15, color: COLORS.textMuted }}>
                  {item.desc}
                </div>
              </div>
            </Sequence>
          );
        })}
      </div>
    </AbsoluteFill>
  );
};
