const brainUrl = "http://localhost:8090/v1/state";

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

function renderOffline() {
  connectionDot.classList.remove("online");
  connectionText.textContent = "Brain unavailable";
}

async function refreshState() {
  try {
    const response = await fetch(brainUrl);

    if (!response.ok) {
      throw new Error("Brain state request failed");
    }

    renderState(await response.json());
  } catch {
    renderOffline();
  }
}

renderDimensions({});
refreshState();
setInterval(refreshState, 1000);