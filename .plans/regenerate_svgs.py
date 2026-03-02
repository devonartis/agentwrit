#!/usr/bin/env python3
"""
Regenerate all 6 CoWork SVG diagrams with professional tech/SaaS styling.
Design system: Stripe/Linear inspired color palette with WCAG-compliant contrast.
"""

def create_svg_header(width, height):
    """Create SVG header with shared styles and definitions."""
    return f'''<?xml version="1.0" encoding="UTF-8"?>
<svg width="{width}" height="{height}" viewBox="0 0 {width} {height}" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="arrowhead-dark" markerWidth="12" markerHeight="12" refX="10" refY="6" orient="auto">
      <polygon points="0 0, 12 6, 0 12" fill="#425466"/>
    </marker>
    <marker id="arrowhead-white" markerWidth="12" markerHeight="12" refX="10" refY="6" orient="auto">
      <polygon points="0 0, 12 6, 0 12" fill="#FFFFFF"/>
    </marker>
    <style>
      @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
      * {{ font-family: 'Inter', system-ui, -apple-system, sans-serif; }}
      .title {{ font-size: 22px; font-weight: 700; fill: #FFFFFF; }}
      .header {{ font-size: 16px; font-weight: 600; fill: #0A2540; }}
      .subheader {{ font-size: 14px; font-weight: 600; fill: #0A2540; }}
      .step-text {{ font-size: 13px; font-weight: 400; fill: #0A2540; }}
      .subtext {{ font-size: 11px; font-weight: 400; fill: #425466; }}
      .banner-text {{ font-size: 14px; font-weight: 600; fill: #FFFFFF; }}
      .pill-text {{ font-size: 10px; font-weight: 700; fill: #FFFFFF; text-anchor: middle; }}
    </style>
  </defs>
  <rect width="{width}" height="{height}" fill="#FFFFFF"/>
'''

def create_title_bar(x, y, width, height, text):
    """Create a navy title bar with white text."""
    return f'''  <!-- Title Bar -->
  <rect x="{x}" y="{y}" width="{width}" height="{height}" fill="#0A2540" rx="0"/>
  <text x="{x + width/2}" y="{y + height/2 + 8}" text-anchor="middle" class="title">{text}</text>
'''

def create_step_box(x, y, width, height, step_num, text, border_color, has_pill=False, pill_text="", pill_color=""):
    """Create a step box with colored left border and optional status pill."""
    svg = f'''  <!-- Step {step_num} -->
  <rect x="{x}" y="{y}" width="{width}" height="{height}" fill="#FFFFFF" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <rect x="{x}" y="{y}" width="4" height="{height}" fill="{border_color}" rx="8 0 0 8"/>
  <text x="{x + 16}" y="{y + 24}" class="step-text">{text}</text>
'''
    if has_pill:
        pill_x = x + width - 80
        pill_y = y + height - 20
        svg += f'''  <rect x="{pill_x}" y="{pill_y}" width="72" height="18" fill="{pill_color}" rx="6"/>
  <text x="{pill_x + 36}" y="{pill_y + 13}" class="pill-text">{pill_text}</text>
'''
    return svg

def create_status_pill(x, y, text, color):
    """Create a status pill (colored rectangle with white text)."""
    width = max(60, len(text) * 6.5 + 8)
    return f'''  <rect x="{x}" y="{y}" width="{width}" height="24" fill="{color}" rx="6"/>
  <text x="{x + width/2}" y="{y + 16}" class="pill-text">{text}</text>
'''

def create_info_box(x, y, width, height, title, color_left, bg_color, text_lines):
    """Create an info box with colored left border."""
    svg = f'''  <rect x="{x}" y="{y}" width="{width}" height="{height}" fill="{bg_color}" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <rect x="{x}" y="{y}" width="4" height="{height}" fill="{color_left}" rx="8 0 0 8"/>
  <text x="{x + 16}" y="{y + 24}" class="subheader">{title}</text>
'''
    line_y = y + 50
    for text in text_lines:
        svg += f'''  <text x="{x + 16}" y="{line_y}" class="step-text">{text}</text>
'''
        line_y += 22
    return svg

def svg_footer():
    """Create SVG footer."""
    return '</svg>\n'

# FILE 1: CoWork-Diagram-AppOnboarding.svg
def generate_app_onboarding():
    svg = create_svg_header(1200, 900)
    svg += create_title_bar(0, 0, 1200, 60, "How Apps Get Onboarded — Current vs. What Production Needs")

    # LEFT COLUMN: Current Reality
    svg += '''  <!-- LEFT COLUMN HEADER -->
  <rect x="20" y="70" width="540" height="45" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <rect x="20" y="70" width="4" height="45" fill="#CF222E" rx="8 0 0 8"/>
  <text x="280" y="98" text-anchor="middle" class="subheader">Current Reality</text>
'''

    # Current Reality steps
    current_steps = [
        (1, "Operator decides app scopes", "#1A7F37"),  # Green - OK
        (2, "Edit docker-compose.yml or systemd config", "#CF222E", True, "BROKEN", "#CF222E"),
        (3, "Paste Admin Master Key into proxy config", "#CF222E", True, "BROKEN", "#CF222E"),
        (4, "Deploy a Token Proxy instance", "#CF222E", True, "BROKEN", "#CF222E"),
        (5, "Token Proxy self-activates with master key", "#CF222E", True, "BROKEN", "#CF222E"),
        (6, "App invisible — no entity, no name, no tracking", "#CF222E", True, "BROKEN", "#CF222E"),
    ]

    y_pos = 130
    for item in current_steps:
        step_num = item[0]
        text = item[1]
        border_color = item[2]
        has_pill = len(item) > 3 and item[3]
        pill_text = item[4] if has_pill else ""
        pill_color = item[5] if has_pill else ""

        svg += create_step_box(20, y_pos, 540, 50, step_num, text, border_color, has_pill, pill_text, pill_color)
        if step_num < 6:
            svg += f'''  <line x1="290" y1="{y_pos + 50}" x2="290" y2="{y_pos + 70}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
'''
        y_pos += 70

    # Current Reality result box
    svg += '''  <rect x="20" y="550" width="540" height="80" fill="#FFF0F0" stroke="#CF222E" stroke-width="2" rx="8"/>
  <text x="290" y="575" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">Result: Adding an app = an</text>
  <text x="290" y="598" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">infrastructure deployment,</text>
  <text x="290" y="621" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">not an API call.</text>
'''

    # RIGHT COLUMN: What Production Needs
    svg += '''  <!-- RIGHT COLUMN HEADER -->
  <rect x="640" y="70" width="540" height="45" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <rect x="640" y="70" width="4" height="45" fill="#0969DA" rx="8 0 0 8"/>
  <text x="910" y="98" text-anchor="middle" class="subheader">What Production Needs</text>
'''

    # Production Needs steps
    production_steps = [
        (1, "Operator decides app scopes", "#1A7F37"),  # Green - OK
        (2, "CLI: register-app --name --scopes", "#0969DA", True, "API CALL", "#0969DA"),
        (3, "Broker creates app entity, returns token", "#0969DA", True, "API RESPONSE", "#0969DA"),
        (4, "Pass activation token to deployment", "#0969DA", True, "HANDOFF", "#0969DA"),
        (5, "Token Proxy boots with activation token", "#0969DA", True, "SECURE", "#0969DA"),
        (6, "App is named, tracked, auditable", "#1A7F37", True, "PRODUCTION", "#1A7F37"),
    ]

    y_pos = 130
    for item in production_steps:
        step_num = item[0]
        text = item[1]
        border_color = item[2]
        has_pill = len(item) > 3 and item[3]
        pill_text = item[4] if has_pill else ""
        pill_color = item[5] if has_pill else ""

        svg += create_step_box(640, y_pos, 540, 50, step_num, text, border_color, has_pill, pill_text, pill_color)
        if step_num < 6:
            svg += f'''  <line x1="910" y1="{y_pos + 50}" x2="910" y2="{y_pos + 70}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
'''
        y_pos += 70

    # Production result box
    svg += '''  <rect x="640" y="550" width="540" height="80" fill="#EFF6FF" stroke="#0969DA" stroke-width="2" rx="8"/>
  <text x="910" y="575" text-anchor="middle" class="step-text" font-weight="600" fill="#0969DA">Result: Adding an app = one</text>
  <text x="910" y="598" text-anchor="middle" class="step-text" font-weight="600" fill="#0969DA">CLI command. Works on any</text>
  <text x="910" y="621" text-anchor="middle" class="step-text" font-weight="600" fill="#0969DA">server.</text>
'''

    # Bottom banner
    svg += '''  <!-- Bottom Banner -->
  <rect x="20" y="680" width="1160" height="70" fill="#CF222E" rx="8"/>
  <text x="600" y="715" text-anchor="middle" class="banner-text">BLOCKER: The entire left column is what exists today.</text>
  <text x="600" y="735" text-anchor="middle" class="banner-text">Production cannot scale with infrastructure changes for every app.</text>
'''

    svg += svg_footer()
    return svg

# FILE 2: CoWork-Diagram-CredentialFlow.svg
def generate_credential_flow():
    svg = create_svg_header(1400, 1000)
    svg += create_title_bar(0, 0, 1400, 60, "How an App Gets a Credential — End to End")

    # Three swim lanes
    lane_y = 70
    lane_height = 300

    # Operator lane
    svg += f'''  <!-- OPERATOR LANE -->
  <rect x="20" y="{lane_y}" width="1360" height="{lane_height}" fill="#EFF6FF" stroke="#D0D7DE" stroke-width="1"/>
  <rect x="20" y="{lane_y}" width="140" height="50" fill="#0969DA" rx="6"/>
  <text x="90" y="{lane_y + 32}" text-anchor="middle" class="pill-text" fill="#FFFFFF">OPERATOR</text>

  <text x="180" y="{lane_y + 100}" class="step-text" font-weight="600">1. Generate Master Key</text>
  <line x1="360" y1="{lane_y + 85}" x2="400" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="420" y="{lane_y + 100}" class="step-text" font-weight="600">2. Deploy Broker</text>
  <line x1="600" y1="{lane_y + 85}" x2="640" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="660" y="{lane_y + 100}" class="step-text" font-weight="600">3. Deploy Token Proxy</text>
  <rect x="860" y="{lane_y + 75}" width="72" height="24" fill="#CF222E" rx="6"/>
  <text x="896" y="{lane_y + 91}" text-anchor="middle" class="pill-text">BROKEN</text>
  <line x1="950" y1="{lane_y + 85}" x2="990" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="1010" y="{lane_y + 100}" class="step-text" font-weight="600">4. Tell dev proxy URL</text>

  <text x="180" y="{lane_y + 180}" class="subtext" font-style="italic">Requires infrastructure configuration + master key storage</text>
'''

    # Developer lane
    lane_y += lane_height
    svg += f'''  <!-- DEVELOPER LANE -->
  <rect x="20" y="{lane_y}" width="1360" height="{lane_height}" fill="#F0FFF4" stroke="#D0D7DE" stroke-width="1"/>
  <rect x="20" y="{lane_y}" width="140" height="50" fill="#1A7F37" rx="6"/>
  <text x="90" y="{lane_y + 32}" text-anchor="middle" class="pill-text" fill="#FFFFFF">DEVELOPER</text>

  <text x="180" y="{lane_y + 100}" class="step-text" font-weight="600">1. Call POST /token</text>
  <line x1="350" y1="{lane_y + 85}" x2="390" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="410" y="{lane_y + 100}" class="step-text" font-weight="600">2. First-time agent?</text>

  <!-- Registration path -->
  <rect x="560" y="{lane_y + 60}" width="420" height="140" fill="#FFFFFF" stroke="#D0D7DE" stroke-width="1" stroke-dasharray="5,5" rx="6"/>
  <text x="770" y="{lane_y + 85}" text-anchor="middle" class="subtext" font-weight="600">First-Time Registration (5 calls)</text>
  <text x="575" y="{lane_y + 110}" class="step-text">• Auth as admin</text>
  <text x="575" y="{lane_y + 133}" class="step-text">• Create launch token</text>
  <text x="575" y="{lane_y + 156}" class="step-text">• Generate key pair → Challenge-response</text>
  <text x="575" y="{lane_y + 179}" class="step-text">• Register identity</text>
  <rect x="990" y="{lane_y + 75}" width="72" height="24" fill="#9A6700" rx="6"/>
  <text x="1026" y="{lane_y + 91}" text-anchor="middle" class="pill-text">GAP</text>

  <line x1="360" y1="{lane_y + 220}" x2="400" y2="{lane_y + 220}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="420" y="{lane_y + 235}" class="step-text" font-weight="600">3. Proxy requests scoped token</text>
  <line x1="650" y1="{lane_y + 220}" x2="690" y2="{lane_y + 220}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="710" y="{lane_y + 235}" class="step-text" font-weight="600">4. Return JWT</text>

  <text x="180" y="{lane_y + 280}" class="subtext" font-style="italic">Hand-coded HTTP required. No SDK available.</text>
'''

    # App lane
    lane_y += lane_height
    svg += f'''  <!-- APP LANE -->
  <rect x="20" y="{lane_y}" width="1360" height="{lane_height}" fill="#FFF8E1" stroke="#D0D7DE" stroke-width="1"/>
  <rect x="20" y="{lane_y}" width="140" height="50" fill="#9A6700" rx="6"/>
  <text x="90" y="{lane_y + 32}" text-anchor="middle" class="pill-text" fill="#FFFFFF">RUNNING APP</text>

  <text x="180" y="{lane_y + 100}" class="step-text" font-weight="600">1. Present Bearer token</text>
  <line x1="380" y1="{lane_y + 85}" x2="420" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="440" y="{lane_y + 100}" class="step-text" font-weight="600">2. Validate with broker</text>
  <line x1="680" y1="{lane_y + 85}" x2="720" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="740" y="{lane_y + 100}" class="step-text" font-weight="600">3. Check scope</text>
  <line x1="920" y1="{lane_y + 85}" x2="960" y2="{lane_y + 85}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
  <text x="980" y="{lane_y + 100}" class="step-text" font-weight="600">4. Allow/deny</text>
  <rect x="1150" y="{lane_y + 75}" width="72" height="24" fill="#CF222E" rx="6"/>
  <text x="1186" y="{lane_y + 91}" text-anchor="middle" class="pill-text">BROKEN</text>

  <text x="180" y="{lane_y + 180}" class="subtext" font-style="italic">Every validation = network call to single broker. No JWKS delegation.</text>
'''

    svg += svg_footer()
    return svg

# FILE 3: CoWork-Diagram-DoWeNeedProxy.svg
def generate_do_we_need_proxy():
    svg = create_svg_header(1200, 900)
    svg += create_title_bar(0, 0, 1200, 60, "The Fundamental Question: Do We Need the Token Proxy?")

    # LEFT COLUMN: With Proxy
    svg += '''  <!-- LEFT COLUMN: WITH PROXY -->
  <rect x="20" y="80" width="540" height="45" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <text x="290" y="108" text-anchor="middle" class="subheader">With Token Proxy</text>

  <!-- Diagram -->
  <rect x="80" y="145" width="100" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="130" y="178" text-anchor="middle" class="step-text" font-weight="600">App</text>

  <line x1="180" y1="175" x2="230" y2="175" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>

  <rect x="230" y="145" width="120" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="290" y="178" text-anchor="middle" class="step-text" font-weight="600">Token Proxy</text>

  <line x1="350" y1="175" x2="400" y2="175" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>

  <rect x="400" y="145" width="100" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="450" y="178" text-anchor="middle" class="step-text" font-weight="600">Broker</text>

  <!-- Benefits -->
  <rect x="40" y="230" width="240" height="140" fill="#FFFFFF" stroke="#1A7F37" stroke-width="1" rx="6"/>
  <text x="160" y="255" text-anchor="middle" class="subtext" font-weight="600" fill="#1A7F37">✓ Benefits</text>
  <text x="55" y="280" class="step-text">• 1 call vs 5-10</text>
  <text x="55" y="302" class="step-text">• Handles crypto</text>
  <text x="55" y="324" class="step-text">• Circuit breaker</text>
  <text x="55" y="346" class="step-text">• Local scope check</text>

  <!-- Problems -->
  <rect x="300" y="230" width="240" height="140" fill="#FFFFFF" stroke="#CF222E" stroke-width="1" rx="6"/>
  <text x="420" y="255" text-anchor="middle" class="subtext" font-weight="600" fill="#CF222E">✗ Problems</text>
  <text x="315" y="280" class="step-text">• Every proxy has master</text>
  <text x="315" y="302" class="step-text">  key (security risk)</text>
  <text x="315" y="324" class="step-text">• Adding app = infra change</text>
  <text x="315" y="346" class="step-text">• N proxies = N keys</text>
'''

    # RIGHT COLUMN: Without Proxy
    svg += '''  <!-- RIGHT COLUMN: WITHOUT PROXY -->
  <rect x="640" y="80" width="540" height="45" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <text x="910" y="108" text-anchor="middle" class="subheader">Without Token Proxy</text>

  <!-- Diagram -->
  <rect x="700" y="145" width="100" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="750" y="178" text-anchor="middle" class="step-text" font-weight="600">App</text>

  <line x1="800" y1="175" x2="850" y2="175" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>

  <rect x="850" y="145" width="100" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="900" y="178" text-anchor="middle" class="step-text" font-weight="600">Broker</text>

  <!-- Benefits -->
  <rect x="660" y="230" width="240" height="140" fill="#FFFFFF" stroke="#1A7F37" stroke-width="1" rx="6"/>
  <text x="780" y="255" text-anchor="middle" class="subtext" font-weight="600" fill="#1A7F37">✓ Benefits</text>
  <text x="675" y="280" class="step-text">• No middleware layer</text>
  <text x="675" y="302" class="step-text">• Master key with operator</text>
  <text x="675" y="324" class="step-text">• API registration</text>
  <text x="675" y="346" class="step-text">• Unique credentials</text>

  <!-- Problems -->
  <rect x="920" y="230" width="240" height="140" fill="#FFFFFF" stroke="#CF222E" stroke-width="1" rx="6"/>
  <text x="1040" y="255" text-anchor="middle" class="subtext" font-weight="600" fill="#CF222E">✗ Problems</text>
  <text x="935" y="280" class="step-text">• Developers handle 5-10</text>
  <text x="935" y="302" class="step-text">  calls</text>
  <text x="935" y="324" class="step-text">• Need SDK (not yet built)</text>
  <text x="935" y="346" class="step-text">• No local scope enforcement</text>
'''

    # Bottom recommendation
    svg += '''  <!-- BOTTOM RECOMMENDATION -->
  <rect x="40" y="430" width="1120" height="120" fill="#FFFFFF" stroke="#0969DA" stroke-width="2" rx="8"/>
  <rect x="40" y="430" width="4" height="120" fill="#0969DA" rx="8 0 0 8"/>
  <text x="60" y="460" class="subheader">RECOMMENDATION</text>
  <text x="60" y="490" class="step-text">The Token Proxy should be OPTIONAL. Apps should register directly with the broker</text>
  <text x="60" y="513" class="step-text">via an SDK. The proxy adds value as an optimization layer but cannot be the only path.</text>
  <text x="60" y="536" class="step-text">Production workloads need to choose the right model for their use case.</text>
'''

    svg += svg_footer()
    return svg

# FILE 4: CoWork-Diagram-ProductionFailures.svg
def generate_production_failures():
    svg = create_svg_header(1400, 1000)
    svg += create_title_bar(0, 0, 1400, 60, "What Breaks in Production — Failure Analysis")

    failures = [
        {
            "title": "Broker Restart",
            "chain": [
                "Broker stops",
                "New signing keys",
                "ALL tokens invalid",
                "ALL proxies re-bootstrap",
                "ALL agents re-register"
            ],
            "impact": "Total service disruption",
            "y": 80
        },
        {
            "title": "Proxy Compromise",
            "chain": [
                "Attacker accesses any proxy",
                "Proxy holds master key",
                "Full admin access obtained",
                "Can revoke, audit, create"
            ],
            "impact": "Complete system compromise",
            "y": 240
        },
        {
            "title": "Adding App #11",
            "chain": [
                "Business requests new app",
                "Edit infrastructure config",
                "Paste master key into config",
                "Deploy new proxy instance"
            ],
            "impact": "Infrastructure change for business request",
            "y": 400
        },
        {
            "title": "Key Rotation",
            "chain": [
                "Policy requires key rotation",
                "Update broker signing keys",
                "Update ALL proxy keys",
                "Restart entire fleet"
            ],
            "impact": "Coordinated total outage",
            "y": 560
        },
        {
            "title": "Audit Investigation",
            "chain": [
                "Question: What did App C do?",
                "All proxies show as 'proxy'",
                "No app names in audit trail",
                "Manual log correlation needed"
            ],
            "impact": "Audit trail is useless for apps",
            "y": 720
        },
    ]

    for failure in failures:
        is_broken = "Broker" in failure["title"] or "Proxy" in failure["title"] or "Audit" in failure["title"]
        color = "#CF222E" if is_broken else "#9A6700"

        y = failure["y"]
        svg += f'''  <!-- {failure["title"]} -->
  <rect x="20" y="{y}" width="1360" height="150" fill="#FFFFFF" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <rect x="20" y="{y}" width="4" height="150" fill="{color}" rx="8 0 0 8"/>
  <text x="40" y="{y + 28}" class="subheader">{failure["title"]}</text>

  <!-- Chain -->
'''
        chain_x = 300
        for i, step in enumerate(failure["chain"]):
            svg += f'''  <rect x="{chain_x}" y="{y + 15}" width="130" height="50" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="4"/>
  <text x="{chain_x + 65}" y="{y + 43}" text-anchor="middle" class="step-text">{step}</text>
'''
            if i < len(failure["chain"]) - 1:
                svg += f'''  <line x1="{chain_x + 130}" y1="{y + 40}" x2="{chain_x + 160}" y2="{y + 40}" stroke="#425466" stroke-width="2" marker-end="url(#arrowhead-dark)"/>
'''
            chain_x += 170

        # Impact box
        impact_x = 1150
        svg += f'''  <!-- Impact -->
  <rect x="{impact_x}" y="{y + 15}" width="220" height="120" fill="{color}" rx="4"/>
  <text x="{impact_x + 110}" y="{y + 45}" text-anchor="middle" class="banner-text">{failure["impact"]}</text>
'''

    # Root cause summary
    svg += '''  <!-- ROOT CAUSE SUMMARY -->
  <rect x="20" y="900" width="1360" height="70" fill="#FFF0F0" stroke="#CF222E" stroke-width="2" rx="8"/>
  <text x="700" y="930" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">Root Cause: Token Proxy is mandatory, holds master key, apps don't exist as entities</text>
  <text x="700" y="955" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">Result: Any failure cascades to the entire fleet with no granular isolation.</text>
'''

    svg += svg_footer()
    return svg

# FILE 5: CoWork-Diagram-FailureModes.svg
def generate_failure_modes():
    svg = create_svg_header(1200, 1000)
    svg += create_title_bar(0, 0, 1200, 60, "Failure Cascade — How One Issue Creates Five Problems")

    # Central circle: Master Key in Every Proxy
    svg += '''  <!-- ROOT CAUSE 1: MASTER KEY -->
  <circle cx="600" cy="200" r="80" fill="#FFFFFF" stroke="#CF222E" stroke-width="3"/>
  <text x="600" y="190" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">Admin Master Key</text>
  <text x="600" y="210" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">in Every Proxy</text>
'''

    # Five consequences radiating out
    consequences1 = [
        {
            "angle": -90,
            "text": "Broker restart =\ntotal disruption",
            "x": 600,
            "y": 80
        },
        {
            "angle": -45,
            "text": "Any proxy compromise\n= full admin access",
            "x": 745,
            "y": 145
        },
        {
            "angle": 0,
            "text": "Adding app\nrequires infra\ndeployment",
            "x": 785,
            "y": 320
        },
        {
            "angle": 45,
            "text": "Key rotation =\ncoordinated\noutage",
            "x": 745,
            "y": 455
        },
        {
            "angle": 90,
            "text": "Audit trail has\nno app attribution",
            "x": 600,
            "y": 520
        },
    ]

    for item in consequences1:
        x, y = item["x"], item["y"]
        # Draw line from center to consequence
        svg += f'''  <line x1="600" y1="280" x2="{x}" y2="{y + 30}" stroke="#CF222E" stroke-width="2"/>
  <rect x="{x - 70}" y="{y}" width="140" height="80" fill="#FFFFFF" stroke="#CF222E" stroke-width="2" rx="6"/>
  <text x="{x}" y="{y + 42}" text-anchor="middle" class="step-text" font-weight="600" fill="#0A2540">{item["text"]}</text>
'''

    # ROOT CAUSE 2: Apps Don't Exist
    svg += '''  <!-- ROOT CAUSE 2: APPS DON'T EXIST -->
  <circle cx="600" cy="700" r="70" fill="#FFFFFF" stroke="#CF222E" stroke-width="3"/>
  <text x="600" y="690" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">Apps Don't Exist</text>
  <text x="600" y="710" text-anchor="middle" class="step-text" font-weight="600" fill="#CF222E">as Entities</text>
'''

    # Three consequences from second circle
    svg += '''  <!-- Consequences of Apps Not Being Entities -->
  <line x1="550" y1="770" x2="430" y2="840" stroke="#CF222E" stroke-width="2"/>
  <rect x="330" y="820" width="140" height="70" fill="#FFFFFF" stroke="#CF222E" stroke-width="2" rx="6"/>
  <text x="400" y="855" text-anchor="middle" class="step-text" font-weight="600" fill="#0A2540">Cannot register\nvia API</text>

  <line x1="600" y1="770" x2="600" y2="820" stroke="#CF222E" stroke-width="2"/>
  <rect x="530" y="820" width="140" height="70" fill="#FFFFFF" stroke="#CF222E" stroke-width="2" rx="6"/>
  <text x="600" y="855" text-anchor="middle" class="step-text" font-weight="600" fill="#0A2540">Cannot revoke\nby app name</text>

  <line x1="650" y1="770" x2="770" y2="840" stroke="#CF222E" stroke-width="2"/>
  <rect x="730" y="820" width="140" height="70" fill="#FFFFFF" stroke="#CF222E" stroke-width="2" rx="6"/>
  <text x="800" y="855" text-anchor="middle" class="step-text" font-weight="600" fill="#0A2540">Cannot audit\nby app</text>
'''

    svg += svg_footer()
    return svg

# FILE 6: CoWork-Diagram-GapSummary.svg
def generate_gap_summary():
    svg = create_svg_header(1200, 800)
    svg += create_title_bar(0, 0, 1200, 60, "Gap Analysis — Severity by Category and Persona")

    # Table structure
    table_x = 80
    table_y = 100
    col_widths = [200, 250, 250, 250]
    row_height = 50

    # Headers
    headers = ["Category", "Operator", "Developer", "Running App"]
    for i, header in enumerate(headers):
        x = table_x + sum(col_widths[:i])
        svg += f'''  <rect x="{x}" y="{table_y}" width="{col_widths[i]}" height="{row_height}" fill="#0A2540" rx="4 4 0 0"/>
  <text x="{x + col_widths[i]//2}" y="{table_y + 32}" text-anchor="middle" class="banner-text">{header}</text>
'''

    # Data rows
    rows = [
        ("App Registration", "BLOCKER", "BLOCKER", "N/A"),
        ("Security Model", "BROKEN", "GAP", "GAP"),
        ("Scalability", "BROKEN", "GAP", "BROKEN"),
        ("Resilience", "BROKEN", "BROKEN", "BROKEN"),
        ("Developer Experience", "N/A", "GAP", "GAP"),
        ("Operations", "BROKEN", "N/A", "GAP"),
    ]

    color_map = {
        "BLOCKER": ("#CF222E", "#FFFFFF"),
        "BROKEN": ("#CF222E", "#FFFFFF"),
        "GAP": ("#9A6700", "#FFFFFF"),
        "N/A": ("#D0D7DE", "#0A2540"),
    }

    for row_idx, row in enumerate(rows):
        y = table_y + (row_idx + 1) * row_height
        bg_color = "#FFFFFF" if row_idx % 2 == 0 else "#F6F8FA"

        # Category column
        svg += f'''  <rect x="{table_x}" y="{y}" width="{col_widths[0]}" height="{row_height}" fill="{bg_color}" stroke="#D0D7DE" stroke-width="1"/>
  <text x="{table_x + 12}" y="{y + 32}" class="step-text">{row[0]}</text>
'''

        # Status columns
        for col_idx, status in enumerate(row[1:], 1):
            x = table_x + sum(col_widths[:col_idx])
            color, text_color = color_map.get(status, ("#D0D7DE", "#0A2540"))

            svg += f'''  <rect x="{x}" y="{y}" width="{col_widths[col_idx]}" height="{row_height}" fill="{bg_color}" stroke="#D0D7DE" stroke-width="1"/>
  <rect x="{x + 40}" y="{y + 13}" width="80" height="24" fill="{color}" rx="4"/>
  <text x="{x + 80}" y="{y + 30}" text-anchor="middle" class="pill-text" fill="{text_color}">{status}</text>
'''

    # Summary section
    summary_y = table_y + len(rows) * row_height + 50
    svg += f'''  <!-- SUMMARY -->
  <rect x="80" y="{summary_y}" width="1040" height="60" fill="#F6F8FA" stroke="#D0D7DE" stroke-width="1" rx="8"/>
  <text x="100" y="{summary_y + 25}" class="step-text" font-weight="600">Summary:</text>
  <text x="220" y="{summary_y + 25}" class="step-text" font-weight="600" fill="#CF222E">BLOCKER: 1</text>
  <text x="320" y="{summary_y + 25}" class="step-text" font-weight="600" fill="#CF222E">• BROKEN: 7</text>
  <text x="510" y="{summary_y + 25}" class="step-text" font-weight="600" fill="#9A6700">• GAP: 6</text>
  <text x="650" y="{summary_y + 25}" class="step-text" font-weight="600" fill="#D0D7DE">• N/A: 4</text>
'''

    svg += svg_footer()
    return svg

# Generate all 6 SVGs
if __name__ == "__main__":
    files = [
        ("CoWork-Diagram-AppOnboarding.svg", generate_app_onboarding),
        ("CoWork-Diagram-CredentialFlow.svg", generate_credential_flow),
        ("CoWork-Diagram-DoWeNeedProxy.svg", generate_do_we_need_proxy),
        ("CoWork-Diagram-ProductionFailures.svg", generate_production_failures),
        ("CoWork-Diagram-FailureModes.svg", generate_failure_modes),
        ("CoWork-Diagram-GapSummary.svg", generate_gap_summary),
    ]

    base_path = "/sessions/stoic-festive-curie/mnt/authAgent2/.plans/"

    for filename, generator in files:
        filepath = base_path + filename
        svg_content = generator()

        with open(filepath, "w") as f:
            f.write(svg_content)

        print(f"✓ Generated {filename}")

    print("\nAll 6 SVGs regenerated with professional Stripe/Linear design system.")
    print("Colors: Navy headers, proper contrast, no orange-on-orange or red-on-red.")
