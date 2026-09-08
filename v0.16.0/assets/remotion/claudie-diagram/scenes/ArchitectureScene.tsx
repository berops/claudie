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

type ServiceDef = {
  name: string;
  icon: string;
  color: string;
  x: number;
  y: number;
  desc: string;
};

const SERVICES: ServiceDef[] = [
  {
    name: "Claudie Operator",
    icon: "🎯",
    color: COLORS.primary,
    x: 540,
    y: 130,
    desc: "CRD Controller",
  },
  {
    name: "Manager",
    icon: "🧠",
    color: COLORS.accent,
    x: 540,
    y: 290,
    desc: "Orchestrator",
  },
  {
    name: "Terraformer",
    icon: "🏗️",
    color: COLORS.secondary,
    x: 220,
    y: 440,
    desc: "Terraform",
  },
  {
    name: "Ansibler",
    icon: "🔧",
    color: COLORS.warning,
    x: 430,
    y: 440,
    desc: "Ansible",
  },
  {
    name: "Kube-eleven",
    icon: "☸️",
    color: COLORS.primary,
    x: 650,
    y: 440,
    desc: "KubeOne",
  },
  {
    name: "Kuber",
    icon: "📦",
    color: COLORS.accent,
    x: 860,
    y: 440,
    desc: "kubectl",
  },
];

const DATA_STORES = [
  { name: "MongoDB", icon: "🗄️", x: 180, y: 290 },
  { name: "NATS", icon: "📨", x: 540, y: 570 },
  { name: "MinIO", icon: "💾", x: 900, y: 290 },
];

const CONNECTIONS: [number, number][] = [
  [0, 1], // Operator -> Manager
  [1, 2], // Manager -> Terraformer
  [1, 3], // Manager -> Ansibler
  [1, 4], // Manager -> Kube-eleven
  [1, 5], // Manager -> Kuber
];

export const ArchitectureScene: React.FC = () => {
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
            background: `linear-gradient(180deg, ${COLORS.accent}, ${COLORS.secondary})`,
          }}
        />
        <div style={{ fontSize: 42, fontWeight: 700, color: COLORS.text }}>
          Microservice Architecture
        </div>
      </div>

      <div style={{ position: "relative", flex: 1 }}>
        {/* Connection lines */}
        <svg
          width="100%"
          height="100%"
          style={{ position: "absolute", top: 0, left: 0 }}
        >
          {CONNECTIONS.map(([from, to], i) => {
            const lineProgress = spring({
              frame,
              fps,
              delay: 20 + i * 8,
              config: { damping: 200 },
            });
            const s = SERVICES[from];
            const e = SERVICES[to];
            return (
              <line
                key={i}
                x1={s.x}
                y1={s.y + 30}
                x2={s.x + (e.x - s.x) * lineProgress}
                y2={s.y + 30 + (e.y - s.y - 30) * lineProgress}
                stroke={COLORS.border}
                strokeWidth="2"
                strokeDasharray="6 4"
                opacity={lineProgress}
              />
            );
          })}
          {/* Manager to data stores */}
          {DATA_STORES.map((ds, i) => {
            const lineProgress = spring({
              frame,
              fps,
              delay: 60 + i * 8,
              config: { damping: 200 },
            });
            const mgr = SERVICES[1];
            return (
              <line
                key={`ds-${i}`}
                x1={mgr.x}
                y1={mgr.y}
                x2={mgr.x + (ds.x - mgr.x) * lineProgress}
                y2={mgr.y + (ds.y - mgr.y) * lineProgress}
                stroke={COLORS.primary}
                strokeWidth="1.5"
                strokeDasharray="4 4"
                opacity={lineProgress * 0.5}
              />
            );
          })}
        </svg>

        {/* Service nodes */}
        {SERVICES.map((svc, i) => {
          const nodeScale = spring({
            frame,
            fps,
            delay: 5 + i * 8,
            config: { damping: 15, stiffness: 120 },
          });
          const nodeOpacity = spring({
            frame,
            fps,
            delay: 5 + i * 8,
            config: { damping: 200 },
          });

          return (
            <Sequence key={svc.name} from={0} layout="none">
              <div
                style={{
                  position: "absolute",
                  left: svc.x - 70,
                  top: svc.y - 30,
                  width: 140,
                  textAlign: "center",
                  opacity: nodeOpacity,
                  transform: `scale(${nodeScale})`,
                }}
              >
                <div
                  style={{
                    width: 60,
                    height: 60,
                    borderRadius: 16,
                    background: `${svc.color}18`,
                    border: `2px solid ${svc.color}66`,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontSize: 28,
                    margin: "0 auto 8px",
                    boxShadow: `0 0 20px ${svc.color}22`,
                  }}
                >
                  {svc.icon}
                </div>
                <div
                  style={{
                    fontSize: 14,
                    fontWeight: 700,
                    color: COLORS.text,
                    lineHeight: 1.3,
                  }}
                >
                  {svc.name}
                </div>
                <div style={{ fontSize: 11, color: COLORS.textMuted }}>
                  {svc.desc}
                </div>
              </div>
            </Sequence>
          );
        })}

        {/* Data stores */}
        {DATA_STORES.map((ds, i) => {
          const dsOpacity = spring({
            frame,
            fps,
            delay: 55 + i * 8,
            config: { damping: 200 },
          });
          return (
            <Sequence key={ds.name} from={0} layout="none">
              <div
                style={{
                  position: "absolute",
                  left: ds.x - 50,
                  top: ds.y - 20,
                  width: 100,
                  textAlign: "center",
                  opacity: dsOpacity,
                }}
              >
                <div
                  style={{
                    fontSize: 24,
                    marginBottom: 4,
                  }}
                >
                  {ds.icon}
                </div>
                <div
                  style={{
                    fontSize: 13,
                    fontWeight: 700,
                    color: COLORS.textMuted,
                    background: COLORS.bgLight,
                    borderRadius: 8,
                    padding: "4px 10px",
                    border: `1px solid ${COLORS.border}`,
                  }}
                >
                  {ds.name}
                </div>
              </div>
            </Sequence>
          );
        })}
      </div>
    </AbsoluteFill>
  );
};
