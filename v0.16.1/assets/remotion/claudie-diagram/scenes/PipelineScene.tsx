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

const PIPELINE_STEPS = [
  {
    name: "Terraformer",
    subtitle: "Provision Infrastructure",
    desc: "Uses Terraform to create VMs, networks, and firewall rules across all cloud providers",
    icon: "🏗️",
    color: COLORS.secondary,
  },
  {
    name: "Ansibler",
    subtitle: "Configure Nodes",
    desc: "Sets up WireGuard VPN, installs dependencies, and configures Envoy load balancers via Ansible",
    icon: "🔧",
    color: COLORS.warning,
  },
  {
    name: "Kube-eleven",
    subtitle: "Deploy Kubernetes",
    desc: "Bootstraps Kubernetes clusters using KubeOne across the provisioned nodes",
    icon: "☸️",
    color: COLORS.primary,
  },
  {
    name: "Kuber",
    subtitle: "Configure Cluster",
    desc: "Deploys storage (Longhorn), networking, and cluster resources via kubectl",
    icon: "📦",
    color: COLORS.accent,
  },
];

export const PipelineScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const headerOpacity = spring({ frame, fps, config: { damping: 200 } });

  // Animated progress bar
  const progressWidth = interpolate(frame, [20, 120], [0, 100], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        padding: 60,
      }}
    >
      {/* Header */}
      <div
        style={{
          opacity: headerOpacity,
          display: "flex",
          alignItems: "center",
          gap: 16,
          marginBottom: 20,
        }}
      >
        <div
          style={{
            width: 8,
            height: 40,
            borderRadius: 4,
            background: `linear-gradient(180deg, ${COLORS.secondary}, ${COLORS.warning})`,
          }}
        />
        <div style={{ fontSize: 42, fontWeight: 700, color: COLORS.text }}>
          Provisioning Pipeline
        </div>
      </div>

      <div
        style={{
          fontSize: 18,
          color: COLORS.textMuted,
          marginBottom: 40,
          opacity: headerOpacity,
        }}
      >
        Pending → Scheduled → Done
      </div>

      {/* Pipeline steps */}
      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        {PIPELINE_STEPS.map((step, i) => {
          const stepDelay = 15 + i * 22;
          const stepProgress = spring({
            frame,
            fps,
            delay: stepDelay,
            config: { damping: 200 },
          });
          const stepScale = spring({
            frame,
            fps,
            delay: stepDelay,
            config: { damping: 15, stiffness: 120 },
          });

          const isActive = frame > (stepDelay + 10) * 1 && frame < (stepDelay + 50) * 1;
          const isDone = frame > (stepDelay + 50) * 1;

          // Arrow connector
          const arrowOpacity =
            i < PIPELINE_STEPS.length - 1
              ? spring({
                  frame,
                  fps,
                  delay: stepDelay + 15,
                  config: { damping: 200 },
                })
              : 0;

          return (
            <Sequence key={step.name} from={0} layout="none">
              <div>
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 24,
                    background: isActive
                      ? `${step.color}12`
                      : COLORS.card,
                    borderRadius: 16,
                    padding: "20px 28px",
                    border: `2px solid ${
                      isActive ? step.color : COLORS.border
                    }`,
                    opacity: stepProgress,
                    transform: `scale(${interpolate(
                      stepScale,
                      [0, 1],
                      [0.95, 1]
                    )})`,
                    transition: "none",
                  }}
                >
                  {/* Step number & icon */}
                  <div
                    style={{
                      width: 56,
                      height: 56,
                      borderRadius: 14,
                      background: `${step.color}22`,
                      border: `2px solid ${step.color}`,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontSize: 28,
                      flexShrink: 0,
                    }}
                  >
                    {step.icon}
                  </div>

                  {/* Text content */}
                  <div style={{ flex: 1 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                      <div
                        style={{
                          fontSize: 22,
                          fontWeight: 700,
                          color: COLORS.text,
                        }}
                      >
                        {step.name}
                      </div>
                      <div
                        style={{
                          fontSize: 13,
                          color: step.color,
                          background: `${step.color}18`,
                          padding: "2px 10px",
                          borderRadius: 6,
                        }}
                      >
                        {step.subtitle}
                      </div>
                    </div>
                    <div
                      style={{
                        fontSize: 14,
                        color: COLORS.textMuted,
                        marginTop: 4,
                        lineHeight: 1.4,
                      }}
                    >
                      {step.desc}
                    </div>
                  </div>

                  {/* Status */}
                  <div
                    style={{
                      fontSize: 14,
                      fontWeight: 700,
                      color: isDone
                        ? COLORS.secondary
                        : isActive
                        ? step.color
                        : COLORS.textMuted,
                      flexShrink: 0,
                    }}
                  >
                    {isDone ? "✓ Done" : isActive ? "● Running" : "○ Pending"}
                  </div>
                </div>

                {/* Arrow connector */}
                {i < PIPELINE_STEPS.length - 1 && (
                  <div
                    style={{
                      display: "flex",
                      justifyContent: "center",
                      opacity: arrowOpacity,
                      height: 20,
                    }}
                  >
                    <div
                      style={{
                        width: 2,
                        height: "100%",
                        background: COLORS.border,
                      }}
                    />
                  </div>
                )}
              </div>
            </Sequence>
          );
        })}
      </div>
    </AbsoluteFill>
  );
};
