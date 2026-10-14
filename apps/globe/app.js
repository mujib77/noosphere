const brainUrl = "http://localhost:8090/v1/state";

const historyUrl = "http://localhost:8090/v1/state/history";
const neuralTrace = document.querySelector("#neural-trace");

const dimensions = [
  ["thermalEnergy", "Thermal energy"],
  ["atmosphericMoisture", "Atmospheric moisture"],
  ["pressureField", "Pressure field"],
  ["windMotion", "Wind motion"],
  ["globalArousal", "Global arousal"],
  ["coherence", "Coherence"],
  ["anomaly", "Anomaly"],
  ["sourceTrust", "Source trust"]
];

const planet = document.querySelector("#planet");
const connectionDot = document.querySelector("#connection-dot");
const connectionText = document.querySelector("#connection-text");
const activityValue = document.querySelector("#activity-value");
const eventID = document.querySelector("#event-id");
const updatedAt = document.querySelector("#updated-at");
const dimensionsRoot = document.querySelector("#dimensions");

function renderDimensions(values) {
  dimensionsRoot.innerHTML = dimensions
    .map(([key, label]) => {
      const value = Math.round((values[key] ?? 0) * 100);

      return `
        <article class="dimension">
          <p class="dimension-name">${label}</p>
          <p class="dimension-value">${value}%</p>
          <div
            class="meter"
            role="progressbar"
            aria-label="${label}"
            aria-valuemin="0"
            aria-valuemax="100"
            aria-valuenow="${value}"
          >
            <div class="meter-fill" style="width: ${value}%"></div>
          </div>
        </article>
      `;
    })
    .join("");
}

function renderState(state) {
  const values = state.dimensions;
  const arousal = values.globalArousal ?? 0;
  const anomaly = values.anomaly ?? 0;

  activityValue.textContent = `${Math.round(arousal * 100)}%`;
  eventID.textContent = state.lastEventId ?? "Waiting for sensory input";
  updatedAt.textContent = state.updatedAt
    ? new Date(state.updatedAt).toLocaleTimeString()
    : "--";

  planet.style.setProperty("--planet-scale", (0.94 + arousal * 0.12).toFixed(3));
  document.documentElement.style.setProperty(
    "--glow-strength",
    (0.25 + arousal + anomaly * 0.7).toFixed(3)
  );

  connectionDot.classList.add("online");
  connectionText.textContent = "Brain linked";
  renderDimensions(values);
}

function tracePath(samples, key) {
  const width = 700;
  const height = 180;
  const padding = 14;
  const chartWidth = width - padding * 2;
  const chartHeight = height - padding * 2;

  return samples
    .map((sample, index) => {
      const value = Math.max(0, Math.min(1, sample[key] ?? 0));
      const x = padding + (chartWidth * index) / Math.max(1, samples.length - 1);
      const y = height - padding - value * chartHeight;
      const command = index === 0 ? "M" : "L";

      return `${command} ${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
}

function renderHistory(samples) {
  const recentSamples = samples.slice(-120);

  if (recentSamples.length === 0) {
    neuralTrace.innerHTML = "";
    return;
  }

  const lastSample = recentSamples[recentSamples.length - 1];
  const grid = [0.2, 0.4, 0.6, 0.8]
    .map((level) => {
      const y = 180 - 14 - level * 152;
      return `<line class="trace-grid" x1="14" y1="${y}" x2="686" y2="${y}"></line>`;
    })
    .join("");

  neuralTrace.innerHTML = `
    ${grid}
    <path class="trace-line trace-arousal" d="${tracePath(recentSamples, "globalArousal")}"></path>
    <path class="trace-line trace-coherence" d="${tracePath(recentSamples, "coherence")}"></path>
    <path class="trace-line trace-anomaly" d="${tracePath(recentSamples, "anomaly")}"></path>
    <circle
      class="trace-dot trace-arousal"
      cx="686"
      cy="${(166 - (lastSample.globalArousal ?? 0) * 152).toFixed(1)}"
      r="4"
      fill="var(--cyan)"
    ></circle>
  `;
}

function renderOffline() {
  connectionDot.classList.remove("online");
  connectionText.textContent = "Brain unavailable";
}

async function refreshState() {
  try {
    const [stateResponse, historyResponse] = await Promise.all([
      fetch(brainUrl),
      fetch(historyUrl)
    ]);

    if (!stateResponse.ok || !historyResponse.ok) {
      throw new Error("Brain state request failed");
    }

    const state = await stateResponse.json();
    const history = await historyResponse.json();

    renderState(state);
    renderHistory(history.samples ?? []);
  } catch {
    renderOffline();
  }
}

renderDimensions({});
refreshState();
setInterval(refreshState, 1000);