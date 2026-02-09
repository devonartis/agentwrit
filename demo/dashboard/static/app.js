/* AgentAuth Demo Dashboard -- vanilla JS controller */

(function () {
    "use strict";

    // -- State --
    var currentMode = "insecure";
    var isRunning = false;
    var eventSource = null;

    // -- DOM refs --
    var btnInsecure = document.getElementById("btn-insecure");
    var btnSecure = document.getElementById("btn-secure");
    var btnRun = document.getElementById("btn-run");
    var btnReset = document.getElementById("btn-reset");
    var statusIndicator = document.getElementById("status-indicator");
    var eventList = document.getElementById("event-list");

    // Agent name -> element ID map.
    var agentMap = {
        "Agent-A": "agent-a",
        "Agent-B": "agent-b",
        "Agent-C": "agent-c"
    };

    // -- Mode toggle --

    window.setMode = function (mode) {
        currentMode = mode;
        btnInsecure.classList.toggle("active", mode === "insecure");
        btnSecure.classList.toggle("active", mode === "secure");
    };

    // -- Run demo --

    window.runDemo = function () {
        if (isRunning) return;

        fetch("/demo/run", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ mode: currentMode })
        })
            .then(function (resp) { return resp.json(); })
            .then(function (data) {
                if (data.status === "started") {
                    setRunning(true);
                    connectSSE();
                    resetAgentBlocks();
                }
            });
    };

    // -- Reset --

    window.resetDemo = function () {
        fetch("/demo/reset", { method: "POST" })
            .then(function (resp) { return resp.json(); })
            .then(function () {
                setRunning(false);
                disconnectSSE();
                clearUI();
            });
    };

    // -- UI helpers --

    function setRunning(running) {
        isRunning = running;
        btnRun.disabled = running;
        statusIndicator.textContent = running ? "Running..." : "Idle";
        statusIndicator.className = running ? "status-running" : "status-idle";
    }

    function clearUI() {
        eventList.innerHTML = '<div class="event-placeholder">Events will appear here when the demo runs.</div>';
        resetAgentBlocks();
        resetAttackCards();
    }

    function resetAgentBlocks() {
        Object.values(agentMap).forEach(function (id) {
            var block = document.getElementById(id);
            if (block) {
                block.className = "agent-block idle";
                var statusEl = document.getElementById(id + "-status");
                if (statusEl) statusEl.textContent = "Idle";
            }
        });
    }

    function resetAttackCards() {
        var cards = document.querySelectorAll(".attack-card");
        cards.forEach(function (card) {
            card.className = "attack-card pending";
            var statusEl = card.querySelector(".attack-status");
            if (statusEl) statusEl.textContent = "Pending";
            var detailEl = card.querySelector(".attack-detail");
            if (detailEl) detailEl.remove();
        });
    }

    // -- SSE --

    function connectSSE() {
        disconnectSSE();
        eventSource = new EventSource("/events/stream");

        eventSource.onmessage = function (e) {
            var event;
            try {
                event = JSON.parse(e.data);
            } catch (_err) {
                return;
            }
            handleEvent(event);
        };

        eventSource.onerror = function () {
            // Reconnection is automatic; no action needed.
        };
    }

    function disconnectSSE() {
        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }
    }

    // -- Event handlers --

    function handleEvent(event) {
        switch (event.type) {
            case "agent_event":
                handleAgentEvent(event.data);
                break;
            case "attack_result":
                handleAttackResult(event.data);
                break;
            case "status":
                handleStatusEvent(event.data);
                break;
        }
        appendEventToStream(event);
    }

    function handleAgentEvent(data) {
        var agentId = agentMap[data.agent_name];
        if (!agentId) return;

        var block = document.getElementById(agentId);
        var statusEl = document.getElementById(agentId + "-status");

        if (data.success) {
            block.className = "agent-block done";
            statusEl.textContent = "Done";
        } else {
            block.className = "agent-block failed";
            statusEl.textContent = "Failed";
        }
    }

    function handleAttackResult(data) {
        // Map attack names to card IDs.
        var nameMap = {
            "credential_theft": "attack-credential_theft",
            "lateral_movement": "attack-lateral_movement",
            "impersonation": "attack-impersonation",
            "escalation": "attack-escalation",
            "privilege_escalation": "attack-escalation",
            "accountability": "attack-accountability"
        };

        var cardId = nameMap[data.name];
        if (!cardId) return;

        var card = document.getElementById(cardId);
        if (!card) return;

        var statusEl = card.querySelector(".attack-status");

        if (data.attack_succeeded) {
            card.className = "attack-card exploited";
            statusEl.textContent = "Exploited";
        } else {
            card.className = "attack-card blocked";
            statusEl.textContent = "Blocked";
        }

        // Add detail text.
        var existing = card.querySelector(".attack-detail");
        if (existing) existing.remove();

        var detail = document.createElement("div");
        detail.className = "attack-detail";
        detail.textContent = data.attempts + " attempts, " + data.successes + " succeeded, " + data.blocked + " blocked";
        card.appendChild(detail);
    }

    function handleStatusEvent(data) {
        if (data.message === "Demo complete") {
            setRunning(false);
            disconnectSSE();
        }
    }

    function appendEventToStream(event) {
        // Remove placeholder.
        var placeholder = eventList.querySelector(".event-placeholder");
        if (placeholder) placeholder.remove();

        var item = document.createElement("div");
        item.className = "event-item";

        // Color class.
        if (event.type === "agent_event") {
            item.className += event.data.success ? " event-success" : " event-failure";
        } else if (event.type === "attack_result") {
            item.className += event.data.attack_succeeded ? " event-failure" : " event-success";
        } else {
            item.className += " event-status";
        }

        // Timestamp.
        var timeStr = "";
        if (event.timestamp) {
            var d = new Date(event.timestamp);
            timeStr = d.toLocaleTimeString();
        }

        // Build content.
        var html = '<span class="event-time">' + timeStr + "</span> ";

        if (event.type === "agent_event") {
            html += '<span class="event-agent">' + escapeHtml(event.data.agent_name) + "</span> ";
            html += '<span class="event-action">' + (event.data.success ? "completed" : "failed") + "</span>";
            if (event.data.detail) {
                html += '<div class="event-result">' + escapeHtml(event.data.detail) + "</div>";
            }
        } else if (event.type === "attack_result") {
            html += '<span class="event-agent">' + escapeHtml(event.data.name) + "</span> ";
            html += '<span class="event-action">' + (event.data.attack_succeeded ? "EXPLOITED" : "BLOCKED") + "</span>";
        } else {
            html += '<span class="event-action">' + escapeHtml(event.data.message || "") + "</span>";
        }

        item.innerHTML = html;

        // Prepend (newest first).
        eventList.insertBefore(item, eventList.firstChild);
    }

    function escapeHtml(text) {
        var div = document.createElement("div");
        div.appendChild(document.createTextNode(text));
        return div.innerHTML;
    }

    // -- Init: check status on load --
    fetch("/demo/status")
        .then(function (resp) { return resp.json(); })
        .then(function (data) {
            if (data.running) {
                setRunning(true);
                connectSSE();
            }
            if (data.mode) {
                window.setMode(data.mode);
            }
        });
})();
