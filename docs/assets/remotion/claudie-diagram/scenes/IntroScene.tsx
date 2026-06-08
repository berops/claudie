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
  weights: ["400", "700", "900"],
  subsets: ["latin"],
});

export const IntroScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const logoScale = spring({ frame, fps, config: { damping: 12 } });
  const titleY = interpolate(
    spring({ frame, fps, delay: 10, config: { damping: 200 } }),
    [0, 1],
    [60, 0]
  );
  const titleOpacity = spring({ frame, fps, delay: 10, config: { damping: 200 } });

  const subtitleOpacity = spring({ frame, fps, delay: 25, config: { damping: 200 } });
  const subtitleY = interpolate(
    spring({ frame, fps, delay: 25, config: { damping: 200 } }),
    [0, 1],
    [30, 0]
  );

  const taglineOpacity = spring({ frame, fps, delay: 45, config: { damping: 200 } });

  // Animated grid background
  const gridOffset = interpolate(frame, [0, 300], [0, 50], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      {/* Animated grid */}
      <AbsoluteFill style={{ opacity: 0.06 }}>
        <svg width="100%" height="100%">
          <defs>
            <pattern
              id="grid"
              width="60"
              height="60"
              patternUnits="userSpaceOnUse"
              patternTransform={`translate(${gridOffset}, ${gridOffset})`}
            >
              <path
                d="M 60 0 L 0 0 0 60"
                fill="none"
                stroke={COLORS.primary}
                strokeWidth="1"
              />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />
        </svg>
      </AbsoluteFill>

      {/* Radial glow */}
      <div
        style={{
          position: "absolute",
          width: 600,
          height: 600,
          borderRadius: "50%",
          background: `radial-gradient(circle, ${COLORS.primary}22 0%, transparent 70%)`,
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
        }}
      />

      {/* Logo / Icon */}
      <div
        style={{
          transform: `scale(${logoScale})`,
          marginBottom: 30,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <svg width="120" height="120" viewBox="0 0 120 120">
          {/* Kubernetes-like wheel */}
          <circle
            cx="60"
            cy="60"
            r="45"
            fill="none"
            stroke={COLORS.primary}
            strokeWidth="3"
          />
          <circle cx="60" cy="60" r="15" fill={COLORS.primary} />
          {[0, 60, 120, 180, 240, 300].map((angle) => {
            const rad = (angle * Math.PI) / 180;
            const x1 = 60 + 15 * Math.cos(rad);
            const y1 = 60 + 15 * Math.sin(rad);
            const x2 = 60 + 45 * Math.cos(rad);
            const y2 = 60 + 45 * Math.sin(rad);
            return (
              <line
                key={angle}
                x1={x1}
                y1={y1}
                x2={x2}
                y2={y2}
                stroke={COLORS.primary}
                strokeWidth="3"
              />
            );
          })}
          {/* Cloud overlay */}
          {[0, 120, 240].map((angle) => {
            const rad = (angle * Math.PI) / 180;
            const cx = 60 + 45 * Math.cos(rad);
            const cy = 60 + 45 * Math.sin(rad);
            return (
              <circle
                key={angle}
                cx={cx}
                cy={cy}
                r="10"
                fill={COLORS.secondary}
                opacity="0.8"
              />
            );
          })}
        </svg>
      </div>

      {/* Title */}
      <Sequence from={0} layout="none">
        <div
          style={{
            fontSize: 90,
            fontWeight: 900,
            color: COLORS.text,
            opacity: titleOpacity,
            transform: `translateY(${titleY}px)`,
            letterSpacing: -2,
          }}
        >
          <span style={{ color: COLORS.primary }}>Claudie</span>
        </div>
      </Sequence>

      {/* Subtitle */}
      <Sequence from={0} layout="none">
        <div
          style={{
            fontSize: 32,
            fontWeight: 400,
            color: COLORS.textMuted,
            opacity: subtitleOpacity,
            transform: `translateY(${subtitleY}px)`,
            marginTop: 16,
          }}
        >
          Cloud-Agnostic Managed Kubernetes
        </div>
      </Sequence>

      {/* Tagline */}
      <Sequence from={0} layout="none">
        <div
          style={{
            fontSize: 22,
            fontWeight: 700,
            color: COLORS.secondary,
            opacity: taglineOpacity,
            marginTop: 24,
            padding: "8px 24px",
            borderRadius: 8,
            border: `1px solid ${COLORS.secondary}44`,
            background: `${COLORS.secondary}11`,
          }}
        >
          Multi-cloud clusters from a single manifest
        </div>
      </Sequence>
    </AbsoluteFill>
  );
};
