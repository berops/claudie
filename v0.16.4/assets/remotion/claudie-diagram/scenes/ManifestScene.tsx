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
import { loadFont as loadMono } from "@remotion/google-fonts/JetBrainsMono";
import { COLORS } from "../colors";

const { fontFamily } = loadFont("normal", {
  weights: ["400", "700"],
  subsets: ["latin"],
});
const { fontFamily: monoFamily } = loadMono("normal", {
  weights: ["400"],
  subsets: ["latin"],
});

const YAML_LINES = [
  { text: "apiVersion: claudie.io/v1beta1", indent: 0 },
  { text: "kind: InputManifest", indent: 0 },
  { text: "metadata:", indent: 0 },
  { text: "name: my-cluster", indent: 1 },
  { text: "spec:", indent: 0 },
  { text: "providers:", indent: 1 },
  { text: "- name: aws-provider", indent: 2, color: COLORS.aws },
  { text: "  region: eu-central-1", indent: 2 },
  { text: "- name: gcp-provider", indent: 2, color: COLORS.gcp },
  { text: "  region: europe-west1", indent: 2 },
  { text: "- name: hetzner-provider", indent: 2, color: COLORS.hetzner },
  { text: "  region: fsn1", indent: 2 },
  { text: "nodePools:", indent: 1 },
  { text: "  control:", indent: 1 },
  { text: "    provider: aws-provider", indent: 2, color: COLORS.aws },
  { text: "    count: 3", indent: 2 },
  { text: "  compute:", indent: 1 },
  { text: "    provider: gcp-provider", indent: 2, color: COLORS.gcp },
  { text: "    count: 5", indent: 2 },
];

export const ManifestScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const headerOpacity = spring({ frame, fps, config: { damping: 200 } });

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        padding: 60,
      }}
    >
      {/* Section header */}
      <div
        style={{
          opacity: headerOpacity,
          display: "flex",
          alignItems: "center",
          gap: 16,
          marginBottom: 40,
        }}
      >
        <div
          style={{
            width: 8,
            height: 40,
            borderRadius: 4,
            background: `linear-gradient(180deg, ${COLORS.primary}, ${COLORS.accent})`,
          }}
        />
        <div style={{ fontSize: 42, fontWeight: 700, color: COLORS.text }}>
          Declarative Infrastructure
        </div>
      </div>

      <div style={{ display: "flex", gap: 40, flex: 1 }}>
        {/* YAML Code Block */}
        <div
          style={{
            flex: 1,
            borderRadius: 16,
            background: COLORS.bgLight,
            border: `1px solid ${COLORS.border}`,
            padding: 30,
            overflow: "hidden",
          }}
        >
          <div
            style={{
              display: "flex",
              gap: 8,
              marginBottom: 20,
            }}
          >
            {["#ff5f57", "#ffbd2e", "#28c840"].map((c) => (
              <div
                key={c}
                style={{
                  width: 12,
                  height: 12,
                  borderRadius: "50%",
                  background: c,
                }}
              />
            ))}
            <span
              style={{
                color: COLORS.textMuted,
                fontSize: 13,
                fontFamily: monoFamily,
                marginLeft: 8,
              }}
            >
              input-manifest.yaml
            </span>
          </div>

          {YAML_LINES.map((line, i) => {
            const lineDelay = 8 + i * 3;
            const lineOpacity = spring({
              frame,
              fps,
              delay: lineDelay,
              config: { damping: 200 },
            });
            const lineX = interpolate(
              spring({
                frame,
                fps,
                delay: lineDelay,
                config: { damping: 200 },
              }),
              [0, 1],
              [-20, 0]
            );

            return (
              <Sequence key={i} from={0} layout="none">
                <div
                  style={{
                    fontFamily: monoFamily,
                    fontSize: 16,
                    color: line.color || COLORS.textMuted,
                    opacity: lineOpacity,
                    transform: `translateX(${lineX}px)`,
                    paddingLeft: line.indent * 20,
                    lineHeight: 1.7,
                    whiteSpace: "pre",
                  }}
                >
                  {line.text}
                </div>
              </Sequence>
            );
          })}
        </div>

        {/* Description side */}
        <div style={{ flex: 0.7, display: "flex", flexDirection: "column", gap: 24 }}>
          {[
            {
              icon: "📄",
              title: "YAML Manifest",
              desc: "Define your entire multi-cloud infrastructure in a single InputManifest CRD",
              delay: 30,
            },
            {
              icon: "🔑",
              title: "Provider Secrets",
              desc: "Cloud credentials stored securely as Kubernetes secrets",
              delay: 50,
            },
            {
              icon: "⚡",
              title: "Instant Changes",
              desc: "Edit the manifest to scale, update, or tear down infrastructure",
              delay: 70,
            },
          ].map((item) => {
            const cardScale = spring({
              frame,
              fps,
              delay: item.delay,
              config: { damping: 200 },
            });
            const cardOpacity = spring({
              frame,
              fps,
              delay: item.delay,
              config: { damping: 200 },
            });

            return (
              <Sequence key={item.title} from={0} layout="none">
                <div
                  style={{
                    background: COLORS.card,
                    borderRadius: 12,
                    padding: 24,
                    border: `1px solid ${COLORS.border}`,
                    opacity: cardOpacity,
                    transform: `scale(${interpolate(cardScale, [0, 1], [0.9, 1])})`,
                  }}
                >
                  <div style={{ fontSize: 28, marginBottom: 8 }}>{item.icon}</div>
                  <div
                    style={{
                      fontSize: 20,
                      fontWeight: 700,
                      color: COLORS.text,
                      marginBottom: 6,
                    }}
                  >
                    {item.title}
                  </div>
                  <div style={{ fontSize: 15, color: COLORS.textMuted, lineHeight: 1.5 }}>
                    {item.desc}
                  </div>
                </div>
              </Sequence>
            );
          })}
        </div>
      </div>
    </AbsoluteFill>
  );
};
