#!/usr/bin/env python3
"""
Generate professional PDF from Architecture Design markdown.
Uses reportlab with SaaS color palette and professional styling.
"""

from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.lib.colors import HexColor, white, black
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, PageBreak, Table, TableStyle,
    Preformatted, KeepTogether, PageTemplate, Frame
)
from reportlab.lib import colors
from reportlab.pdfgen import canvas
from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_JUSTIFY, TA_RIGHT
from datetime import datetime
import os

# Color palette
COLOR_NAVY = HexColor("#0A2540")
COLOR_TEAL = HexColor("#00D4AA")
COLOR_CORAL = HexColor("#E25950")
COLOR_PURPLE = HexColor("#635BFF")
COLOR_BODY = HexColor("#425466")
COLOR_LIGHT_GRAY = HexColor("#F7F8FA")

# Page dimensions
PAGE_WIDTH, PAGE_HEIGHT = letter
MARGIN_LEFT = 0.75 * inch
MARGIN_RIGHT = 0.75 * inch
MARGIN_TOP = 0.75 * inch
MARGIN_BOTTOM = 0.75 * inch
CONTENT_WIDTH = PAGE_WIDTH - MARGIN_LEFT - MARGIN_RIGHT

class PageNumberCanvas(canvas.Canvas):
    """Canvas subclass that adds page numbers to every page."""

    def __init__(self, *args, **kwargs):
        canvas.Canvas.__init__(self, *args, **kwargs)
        self.pages = []

    def showPage(self):
        self.pages.append(dict(self.__dict__))
        self._startPage()

    def save(self):
        page_count = len(self.pages)
        for page_num, page in enumerate(self.pages, 1):
            self.__dict__.update(page)
            self.draw_page_number(page_num, page_count)
            canvas.Canvas.showPage(self)
        canvas.Canvas.save(self)

    def draw_page_number(self, page_num, total_pages):
        """Draw page number at bottom right."""
        self.setFont("Helvetica", 10)
        self.setFillColor(COLOR_BODY)
        page_text = f"Page {page_num} of {total_pages}"
        self.drawRightString(PAGE_WIDTH - 0.5 * inch, 0.4 * inch, page_text)

def create_title_page(story, styles):
    """Create the title page with navy background."""
    # Title
    title_style = ParagraphStyle(
        'TitlePage',
        parent=styles['Normal'],
        fontSize=36,
        textColor=COLOR_NAVY,
        alignment=TA_CENTER,
        spaceAfter=24,
        fontName='Helvetica-Bold',
    )
    story.append(Paragraph("Architecture Design", title_style))
    story.append(Spacer(1, 0.1 * inch))
    story.append(Paragraph("Direct-to-Broker App Registration", title_style))

    story.append(Spacer(1, 1.5 * inch))

    # Subtitle
    subtitle_style = ParagraphStyle(
        'Subtitle',
        parent=styles['Normal'],
        fontSize=16,
        textColor=COLOR_BODY,
        alignment=TA_CENTER,
        spaceAfter=12,
    )
    story.append(Paragraph(
        "Ephemeral Agent Credentialing Pattern v1.2",
        subtitle_style
    ))
    story.append(Spacer(1, 0.5 * inch))

    # Date
    date_style = ParagraphStyle(
        'DateStyle',
        parent=styles['Normal'],
        fontSize=12,
        textColor=COLOR_BODY,
        alignment=TA_CENTER,
    )
    story.append(Paragraph(f"Date: {datetime.now().strftime('%B %d, %Y')}", date_style))
    story.append(PageBreak())

def create_styles():
    """Create custom paragraph styles."""
    styles = getSampleStyleSheet()

    # Override Normal style
    styles['Normal'].fontSize = 11
    styles['Normal'].textColor = COLOR_BODY
    styles['Normal'].alignment = TA_JUSTIFY
    styles['Normal'].spaceAfter = 12
    styles['Normal'].fontName = 'Helvetica'

    # Heading 1
    styles['Heading1'].fontSize = 18
    styles['Heading1'].textColor = COLOR_NAVY
    styles['Heading1'].spaceAfter = 12
    styles['Heading1'].spaceBefore = 12
    styles['Heading1'].fontName = 'Helvetica-Bold'

    # Heading 2
    styles['Heading2'].fontSize = 14
    styles['Heading2'].textColor = COLOR_NAVY
    styles['Heading2'].spaceAfter = 10
    styles['Heading2'].spaceBefore = 10
    styles['Heading2'].fontName = 'Helvetica-Bold'

    # Custom styles
    styles.add(ParagraphStyle(
        name='BlockQuote',
        parent=styles['Normal'],
        leftIndent=0.5 * inch,
        fontSize=11,
        textColor=COLOR_BODY,
        borderColor=COLOR_TEAL,
        borderWidth=2,
        borderPadding=12,
        borderRadius=2,
    ))

    styles.add(ParagraphStyle(
        name='CodeStyle',
        parent=styles['Normal'],
        fontName='Courier',
        fontSize=9,
        textColor=black,
        leftIndent=0.25 * inch,
        rightIndent=0.25 * inch,
        backColor=COLOR_LIGHT_GRAY,
    ))

    return styles

def add_capability_summary_table(story, styles):
    """Add the capability preservation summary table with color coding."""
    story.append(Paragraph("Capability Preservation Summary", styles['Heading2']))
    story.append(Spacer(1, 0.2 * inch))

    # Table data with status
    data = [
        ["#", "Capability", "Status", "Details"],
        ["1", "App Registration", "NEW", "Apps become first-class entities with client_id/client_secret"],
        ["2", "App Authentication", "NEW", "Scoped app JWT replaces admin master key"],
        ["3", "Launch Token Creation", "PRESERVED", "Same endpoint, same logic, different auth credential"],
        ["4", "Ed25519 Challenge-Response", "PRESERVED", "Zero changes — core identity mechanism untouched"],
        ["5", "SPIFFE ID Generation", "PRESERVED", "Zero changes — same format, same validation"],
        ["6", "JWT Issuance", "ENHANCED", "Same signing, adds app_id + app_name claims"],
        ["7", "Token Verify/Renew/Release", "PRESERVED", "Zero changes to lifecycle operations"],
        ["8", "Scope Format & Attenuation", "PRESERVED", "Zero changes — action:resource:identifier unchanged"],
        ["9", "Scope Enforcement Middleware", "PRESERVED", "Zero changes — ValMw checks every request identically"],
        ["10", "Delegation Chains", "PRESERVED", "Zero changes — SHA-256 chain hash, depth cap 5"],
        ["11", "4-Level Revocation", "PRESERVED", "Token, agent, task, chain — all unchanged"],
        ["12", "App-Level Revocation", "NEW", "5th level — revoke all tokens by app_id"],
        ["13", "Hash-Chained Audit Trail", "ENHANCED", "Adds app_id attribution, new event types"],
        ["14", "SQLite Persistence", "ENHANCED", "New apps table, app_id column on tokens"],
        ["15", "Admin Operations", "PRESERVED", "All existing admin endpoints unchanged"],
        ["16", "aactl CLI", "ENHANCED", "New app subcommand, existing commands unchanged"],
        ["17", "Health & Metrics", "PRESERVED", "Zero changes"],
    ]

    col_widths = [0.5 * inch, 2.2 * inch, 1.2 * inch, 2.6 * inch]
    table = Table(data, colWidths=col_widths)

    # Color cells based on status
    table_style = [
        ('BACKGROUND', (0, 0), (-1, 0), COLOR_NAVY),
        ('TEXTCOLOR', (0, 0), (-1, 0), white),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 0), (-1, 0), 10),
        ('FONTSIZE', (0, 1), (-1, -1), 9),
        ('GRID', (0, 0), (-1, -1), 0.5, colors.grey),
        ('TOPPADDING', (0, 0), (-1, -1), 6),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 6),
        ('LEFTPADDING', (0, 0), (-1, -1), 6),
        ('RIGHTPADDING', (0, 0), (-1, -1), 6),
    ]

    # Color code the Status column
    for i, row in enumerate(data[1:], 1):
        status = row[2]
        if status == "NEW":
            table_style.append(('BACKGROUND', (2, i), (2, i), COLOR_PURPLE))
            table_style.append(('TEXTCOLOR', (2, i), (2, i), white))
        elif status == "PRESERVED":
            table_style.append(('BACKGROUND', (2, i), (2, i), COLOR_TEAL))
            table_style.append(('TEXTCOLOR', (2, i), (2, i), white))
        elif status == "ENHANCED":
            table_style.append(('BACKGROUND', (2, i), (2, i), COLOR_CORAL))
            table_style.append(('TEXTCOLOR', (2, i), (2, i), white))

    table.setStyle(TableStyle(table_style))
    story.append(table)
    story.append(Spacer(1, 0.3 * inch))

def add_security_invariants(story, styles):
    """Add the 10 security invariants as a numbered list."""
    story.append(Paragraph("The 10 Security Invariants — All Maintained", styles['Heading2']))
    story.append(Spacer(1, 0.2 * inch))

    invariants = [
        "Every agent has a unique, cryptographically-bound identity — Ed25519 keypair + SPIFFE ID. Unchanged.",
        "Every token is signed with Ed25519 — broker's signing key. Unchanged.",
        "Every token has a JTI for individual revocation — UUID per token. Unchanged.",
        "Scopes can only narrow, never widen — attenuation enforced at issuance and delegation. Unchanged.",
        "Delegation chains have bounded depth — max 5 levels. Unchanged.",
        "Delegation chains are tamper-evident — SHA-256 hash linking. Unchanged.",
        "Every revocation is immediate and persistent — written to SQLite, checked on every request. Unchanged.",
        "Every operation generates an audit event — append-only, hash-chained. Enhanced (app attribution added).",
        "Nonces are single-use and time-limited — 30s TTL, consumed on use. Unchanged.",
        "Launch tokens are single-use with scope ceilings — consumed on registration. Unchanged.",
    ]

    for i, invariant in enumerate(invariants, 1):
        # Use checkmark symbol and format as list item
        item_style = ParagraphStyle(
            'ListItem',
            parent=styles['Normal'],
            leftIndent=0.3 * inch,
            bulletIndent=0.2 * inch,
            spaceAfter=8,
        )
        story.append(Paragraph(f"<b>{i}.</b> {invariant}", item_style))

    story.append(Spacer(1, 0.3 * inch))

def main():
    """Generate the PDF document."""
    output_path = "/sessions/stoic-festive-curie/mnt/authAgent2/.plans/CoWork-Architecture-Direct-Broker.pdf"

    # Ensure directory exists
    os.makedirs(os.path.dirname(output_path), exist_ok=True)

    # Create document
    doc = SimpleDocTemplate(
        output_path,
        pagesize=letter,
        rightMargin=MARGIN_RIGHT,
        leftMargin=MARGIN_LEFT,
        topMargin=MARGIN_TOP,
        bottomMargin=MARGIN_BOTTOM,
        canvasmaker=PageNumberCanvas,
    )

    # Create styles
    styles = create_styles()

    # Build story
    story = []

    # Title page
    create_title_page(story, styles)

    # Section: The Question Divine Asked
    story.append(Paragraph("The Question Divine Asked", styles['Heading1']))
    story.append(Spacer(1, 0.1 * inch))
    story.append(Paragraph(
        '"Could we not still have a register app and it give the user an app_id with client_id or client_secret or all three that just says you can use the broker... all the other things for agents security is still there but separate. The reason we initially had the sidecar was so the 3rd party developer doesn\'t have to talk to the broker but why shouldn\'t it talk to the broker?"',
        styles['BlockQuote']
    ))
    story.append(Spacer(1, 0.2 * inch))
    story.append(Paragraph(
        "This is the right question. Let's think it through completely.",
        styles['Normal']
    ))
    story.append(PageBreak())

    # Section: Pattern Compliance First
    story.append(Paragraph("Pattern Compliance First", styles['Heading1']))
    story.append(Spacer(1, 0.1 * inch))
    story.append(Paragraph(
        "Before redesigning anything, we checked whether removing the mandatory sidecar violates the Ephemeral Agent Credentialing Pattern v1.2. It does not.",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("<b>What the pattern REQUIRES:</b>", styles['Normal']))
    requirements = [
        "Ephemeral identity per agent instance (Ed25519 keypair + SPIFFE ID)",
        "Cryptographic signatures on all tokens (Ed25519 or RSA)",
        "JWT with required claims: sub, aud, exp, jti, scope",
        "Immutable audit logging (append-only, tamper-proof)",
        "Scope validation (task-scoped authorization)",
        "mTLS between agent and server",
    ]
    for req in requirements:
        story.append(Paragraph(f"• {req}", styles['Normal']))

    story.append(Spacer(1, 0.2 * inch))

    # Add explanation of why AgentAuth doesn't need SPIRE
    story.append(Paragraph(
        "<b>What we use instead of SPIRE (and why it's pattern-compliant):</b>",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.1 * inch))

    compliance_text = """The pattern references SPIRE as one example of an identity provider, but explicitly allows "or equivalent identity provider." AgentAuth opted out of SPIRE specifically because of the heavy infrastructure it requires — SPIRE needs a dedicated server, sidecar agents on every node, a certificate authority, and a persistence layer just to issue identities. That's exactly the kind of infrastructure overhead we're trying to avoid.

Instead, AgentAuth implements a <b>self-contained Ed25519 challenge-response identity system</b> built directly into the broker:"""

    story.append(Paragraph(compliance_text, styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    identity_steps = [
        "Agent generates an Ed25519 keypair in memory (ephemeral, per-instance)",
        "Broker issues a cryptographic nonce (64-char hex, 30-second TTL, single-use)",
        "Agent signs the nonce with its Ed25519 private key",
        "Broker verifies the signature against the public key",
        "Broker generates a SPIFFE-format ID: <font face=\"Courier\">spiffe://trust-domain/agent/{orchID}/{taskID}/{instanceID}</font>",
        "Broker issues a scoped, short-lived JWT bound to that identity",
    ]

    for i, step in enumerate(identity_steps, 1):
        story.append(Paragraph(f"{i}. {step}", styles['Normal']))

    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph(
        "This satisfies the pattern because:",
        styles['Normal']
    ))

    satisfies = [
        "<b>Ephemeral identity</b> — each agent instance gets a unique cryptographically-bound identity (same as SPIRE)",
        "<b>Cryptographic attestation</b> — Ed25519 signature verification proves the agent holds the private key (same as SPIRE's X.509 SVIDs, just simpler)",
        "<b>SPIFFE-format IDs</b> — agent identities follow the standard SPIFFE path format (interoperable)",
        "<b>No external infrastructure</b> — no SPIRE server, no workload attestation agents, no certificate authority, no PKI chain",
    ]

    for item in satisfies:
        story.append(Paragraph(f"• {item}", styles['Normal']))

    story.append(PageBreak())

    # Continue with What the Sidecar Actually Does section
    story.append(Paragraph("What the Sidecar Actually Does (and What We Keep)", styles['Heading1']))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph(
        "The Token Proxy currently serves five functions. Let's evaluate each:",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.2 * inch))

    # Sidecar functions table
    sidecar_data = [
        ["Function", "What It Does", "Stays in Broker?", "Needs SDK?"],
        ["API simplifier", "Turns 5 broker calls into 1 /v1/token call", "N/A — broker already has the endpoints", "Yes — SDK wraps the multi-call flow"],
        ["Ed25519 key manager", "Generates keypairs for agents, stores them locally", "N/A — this is an app-side concern", "Yes — SDK handles key generation"],
        ["Scope enforcer", "Checks scope ceiling locally before calling broker", "Stays — broker enforces scope at registration", "SDK validates locally too"],
        ["Circuit breaker", "Caches tokens, serves stale when broker is down", "Stays as optional middleware", "SDK can cache too"],
        ["Master key holder", "Authenticates as admin for every new agent", "REMOVED — this is the problem we're solving", "App uses scoped credentials instead"],
    ]

    sidecar_table = Table(sidecar_data, colWidths=[1.2 * inch, 1.5 * inch, 1.4 * inch, 1.4 * inch])
    sidecar_table.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, 0), COLOR_NAVY),
        ('TEXTCOLOR', (0, 0), (-1, 0), white),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 1), (-1, -1), 8),
        ('GRID', (0, 0), (-1, -1), 0.5, colors.grey),
        ('ROWBACKGROUNDS', (0, 1), (-1, -1), [white, COLOR_LIGHT_GRAY]),
        ('TOPPADDING', (0, 0), (-1, -1), 6),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 6),
    ]))

    story.append(sidecar_table)
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph(
        "<b>The sidecar's value is convenience, not security.</b> Every security function (identity verification, scope enforcement, revocation, audit) happens in the broker. The sidecar is a DX layer that got promoted to a mandatory dependency.",
        styles['Normal']
    ))

    story.append(PageBreak())

    # The Proposed Architecture section
    story.append(Paragraph("The Proposed Architecture: Apps as First-Class Entities", styles['Heading1']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("The Core Idea", styles['Heading2']))
    story.append(Paragraph(
        "Think of it like this analogy: today, the only way to get into the building (broker) is through a specific lobby desk (sidecar) that happens to have a copy of the master key. What we're proposing is: give each company (app) their own badge (credentials) that opens the front door directly. The lobby desk can still exist for convenience, but it's not the only entrance.",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("How App Registration Works", styles['Heading2']))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph("<b>Step 1: Operator registers the app</b>", styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    # Code block
    code_block = "aactl app register --name \"weather-bot\" --scopes \"read:weather:*,write:logs:*\""
    story.append(Preformatted(code_block, styles['CodeStyle']))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph(
        "The broker creates:<br/>• An <b>App Record</b> (app_id, name, scope ceiling, status, created_at)<br/>• A <b>Client ID</b> (unique identifier, like <font face=\"Courier\">app-weather-bot-a1b2c3</font>)<br/>• A <b>Client Secret</b> (random 64-char hex, stored hashed in broker)<br/>• An <b>Activation Token</b> (single-use JWT for initial bootstrap, optional)",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("The operator receives:", styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    json_response = """{
  "app_id": "app-weather-bot-a1b2c3",
  "client_id": "wb-a1b2c3d4e5f6",
  "client_secret": "sk_live_...",
  "activation_token": "eyJ...",
  "scopes": ["read:weather:*", "write:logs:*"],
  "broker_url": "https://broker.company.com"
}"""

    story.append(Preformatted(json_response, styles['CodeStyle']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("<b>Step 2: Developer configures the app</b>", styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph("<b>Option A — Using the Python SDK (recommended):</b>", styles['Normal']))
    story.append(Spacer(1, 0.05 * inch))

    sdk_code = """from agentauth import AgentAuthClient

client = AgentAuthClient(
    broker_url="https://broker.company.com",
    client_id="wb-a1b2c3d4e5f6",
    client_secret="sk_live_..."
)

# Get a token for an agent
token = client.get_token(
    agent_name="forecast-agent",
    scope=["read:weather:current"]
)"""

    story.append(Preformatted(sdk_code, styles['CodeStyle']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("<b>Option B — Using the Token Proxy (optional, for resilience):</b>", styles['Normal']))
    story.append(Spacer(1, 0.05 * inch))

    proxy_code = "AA_ACTIVATION_TOKEN=eyJ... AA_BROKER_URL=https://broker.company.com ./token-proxy"
    story.append(Preformatted(proxy_code, styles['CodeStyle']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("<b>Option C — Direct HTTP (no SDK, no proxy):</b>", styles['Normal']))
    story.append(Spacer(1, 0.05 * inch))

    http_code = """# Authenticate the app
curl -X POST https://broker.company.com/v1/app/auth \\
  -d '{"client_id": "wb-a1b2c3d4e5f6", "client_secret": "sk_live_..."}'

# Register an agent (challenge-response)
curl -X POST https://broker.company.com/v1/challenge \\
  -H "Authorization: Bearer $APP_TOKEN" \\
  -d '{"agent_name": "forecast-agent"}'

# ... complete challenge-response, get agent token"""

    story.append(Preformatted(http_code, styles['CodeStyle']))

    story.append(PageBreak())

    # What Stays the Same
    story.append(Paragraph("What Stays the Same", styles['Heading2']))
    story.append(Paragraph(
        "Everything about agent security stays identical:",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.1 * inch))

    preserved_items = [
        "<b>Ed25519 challenge-response:</b> Agents still generate keypairs, sign nonces, prove identity. The pattern's core security mechanism is untouched.",
        "<b>SPIFFE IDs:</b> Agents still get <font face=\"Courier\">spiffe://trust-domain/agent/orch/task/instance</font> identities.",
        "<b>Scope attenuation:</b> Scopes still narrow one-way. An app with <font face=\"Courier\">read:weather:*</font> cannot grant <font face=\"Courier\">write:weather:*</font>.",
        "<b>Short-lived JWTs:</b> Tokens still expire in minutes, not hours.",
        "<b>Delegation chains:</b> Multi-agent delegation still works with cryptographic chain verification.",
        "<b>4-level revocation:</b> Token, agent, task, and chain revocation all still work — and now you can also revoke by app.",
        "<b>Hash-chained audit trail:</b> Still tamper-evident, still persistent — and now you can query by app name.",
    ]

    for item in preserved_items:
        story.append(Paragraph(f"• {item}", styles['Normal']))

    story.append(Spacer(1, 0.3 * inch))

    # What Changes
    story.append(Paragraph("What Changes", styles['Heading2']))
    story.append(Spacer(1, 0.1 * inch))

    changes_data = [
        ["Concern", "Today", "Proposed"],
        ["App identity", "Does not exist", "App entity in broker with name, scopes, credentials"],
        ["How apps authenticate", "Token Proxy uses master key", "Apps use client_id + client_secret (scoped)"],
        ["Master key location", "In every proxy config", "Only on the broker and in the operator's vault"],
        ["Adding an app", "Edit infrastructure config", "API call: POST /v1/admin/apps"],
        ["Removing an app", "Remove infrastructure", "API call: DELETE /v1/admin/apps/{id}"],
        ["Listing apps", "Impossible", "GET /v1/admin/apps or aactl app list"],
        ["Audit by app", "Impossible (all sidecar)", "Query by app_id: what did weather-bot do?"],
        ["Token Proxy", "Mandatory", "Optional — use for DX/resilience, not required"],
        ["Developer experience", "Hand-code HTTP to proxy", "SDK: client.get_token(scope=[...])"],
        ["Security review", "Fails (master key everywhere)", "Passes (scoped credentials, no master key spread)"],
    ]

    changes_table = Table(changes_data, colWidths=[1.8 * inch, 2 * inch, 2.2 * inch])
    changes_table.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, 0), COLOR_NAVY),
        ('TEXTCOLOR', (0, 0), (-1, 0), white),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 1), (-1, -1), 8),
        ('GRID', (0, 0), (-1, -1), 0.5, colors.grey),
        ('ROWBACKGROUNDS', (0, 1), (-1, -1), [white, COLOR_LIGHT_GRAY]),
        ('TOPPADDING', (0, 0), (-1, -1), 6),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 6),
    ]))

    story.append(changes_table)

    story.append(PageBreak())

    # How This Solves Every Problem
    story.append(Paragraph("How This Solves Every Problem We Found", styles['Heading1']))
    story.append(Spacer(1, 0.2 * inch))

    problems = [
        ("Problem 1: Apps don't exist", "SOLVED", "Apps are now first-class entities. Register, list, update, revoke, audit — all by app name."),
        ("Problem 2: Token Proxy is mandatory", "SOLVED", "Three paths: SDK (direct), Token Proxy (optional), raw HTTP (possible). The proxy adds value but isn't required."),
        ("Problem 3: Master key everywhere", "SOLVED", "Apps authenticate with their own client_id/client_secret. The master key stays with the operator. Period. If an app's credentials are compromised, you revoke that ONE app — not rebuild the entire system."),
        ("Problem 4: Broker restart destroys everything", "PARTIALLY SOLVED", "This is a separate problem from app registration. The architecture change doesn't fix ephemeral signing keys directly. BUT: with an SDK handling renewal and with app-level credentials (not master key), recovery after restart is cleaner."),
        ("Problem 5: Audit trail useless", "SOLVED", "Every token now carries app_id in its claims. Every audit event records which app caused it. What did weather-bot do last Tuesday? becomes a simple query."),
        ("Problem 6: Key rotation is a coordinated outage", "MOSTLY SOLVED", "Since apps have their own credentials (not the master key), rotating the ADMIN master key only affects the operator's access — not every app in the system."),
        ("Problem 7: Token validation is a single point of failure", "NOT SOLVED BY THIS", "This still needs a JWKS endpoint (separate enhancement). But it's independent of the sidecar question."),
    ]

    for problem, status, description in problems:
        story.append(Paragraph(f"<b>{problem}</b>", styles['Heading2']))
        story.append(Paragraph(f"<font color='#{COLOR_CORAL.hexval()}' size=11><b>{status}</b></font>", styles['Normal']))
        story.append(Paragraph(description, styles['Normal']))
        story.append(Spacer(1, 0.15 * inch))

    story.append(PageBreak())

    # What Needs to Be Built
    story.append(Paragraph("What Needs to Be Built", styles['Heading1']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("Phase 1: App Registration (P0 — unblocks everything)", styles['Heading2']))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph("<b>Broker changes:</b>", styles['Normal']))
    story.append(Spacer(1, 0.1 * inch))

    broker_changes = [
        "<b>1. New data model: AppRecord</b><br/>Fields: app_id, name, client_id, client_secret_hash, scope_ceiling, status, created_at, updated_at, created_by<br/>Stored in SQLite (persistent, survives restarts)",
        "<b>2. New service: AppSvc</b><br/>RegisterApp(name, scopes, createdBy) — creates AppRecord, generates client_id + client_secret<br/>AuthenticateApp(clientID, clientSecret) — validates credentials, returns scoped JWT<br/>ListApps() — returns all registered apps<br/>UpdateApp(appID, newScopes) — updates scope ceiling<br/>DeregisterApp(appID) — revokes all tokens, marks inactive<br/>RevokeApp(appID) — revokes all tokens for this app immediately",
        "<b>3. New handler endpoints:</b><br/>POST /v1/admin/apps — register a new app<br/>GET /v1/admin/apps — list all apps<br/>GET /v1/admin/apps/{id} — get app details<br/>PUT /v1/admin/apps/{id} — update app scopes<br/>DELETE /v1/admin/apps/{id} — deregister app<br/>POST /v1/app/auth — app authenticates with client_id + client_secret",
        "<b>4. New CLI commands:</b><br/>aactl app register --name NAME --scopes SCOPES<br/>aactl app list<br/>aactl app update --id ID --scopes SCOPES<br/>aactl app remove --id ID<br/>aactl app revoke --id ID",
        "<b>5. Token claims extension:</b><br/>Add app_id and app_name to TknClaims<br/>Backward-compatible (empty for legacy tokens)",
        "<b>6. Audit trail extension:</b><br/>All app-level operations recorded<br/>Agent tokens carry app_id — audit events attributable to apps",
    ]

    for change in broker_changes:
        story.append(Paragraph(f"• {change}", styles['Normal']))
        story.append(Spacer(1, 0.1 * inch))

    story.append(PageBreak())

    # Full Pattern Flow section
    story.append(Paragraph("Full Pattern Flow: How Every Capability Works End-to-End", styles['Heading1']))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph(
        "The architecture change (apps as first-class entities, optional sidecar) only touches HOW apps authenticate to the broker. Everything downstream — the entire Ephemeral Agent Credentialing Pattern — stays identical. This section proves it by walking through every capability the system implements and showing what's preserved vs. what changes.",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.2 * inch))

    story.append(Paragraph("The Complete Agent Lifecycle", styles['Heading2']))
    story.append(Spacer(1, 0.1 * inch))

    lifecycle_diagram = """App Registration -> App Auth -> Launch Token -> Agent Identity -> Token Issuance
     (NEW)          (NEW)     (preserved)    (preserved)      (preserved)
                                    |
Scope Enforcement <- Delegation <- Token Use -> Revocation -> Audit Trail
   (preserved)      (preserved)  (preserved)  (enhanced)   (enhanced)"""

    story.append(Preformatted(lifecycle_diagram, ParagraphStyle(
        'DiagramStyle',
        parent=styles['Normal'],
        fontName='Courier',
        fontSize=8,
    )))

    story.append(Spacer(1, 0.3 * inch))

    # Stage 1: App Registration
    story.append(Paragraph("Stage 1: App Registration (NEW)", styles['Heading2']))
    story.append(Paragraph(
        "<b>Today:</b> Apps don't exist as entities. An operator deploys a sidecar with the admin master key baked into its config. \"Registration\" means infrastructure deployment.",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph(
        "<b>Proposed:</b> Operator runs <font face=\"Courier\">aactl app register --name \"weather-bot\" --scopes \"read:weather:*\"</font>. The broker creates an AppRecord with client_id, client_secret (hashed), and a scope ceiling. The operator gives the developer these credentials. No infrastructure deployed. No master key shared.",
        styles['Normal']
    ))
    story.append(Spacer(1, 0.1 * inch))

    story.append(Paragraph(
        "<b>Pattern requirement met:</b> The pattern describes registration with client_id, client_name, client_uri, and token_endpoint_auth_method. This is exactly what we're implementing.",
        styles['Normal']
    ))

    story.append(PageBreak())

    # Capability Preservation Summary Table
    add_capability_summary_table(story, styles)

    # Security Invariants
    add_security_invariants(story, styles)

    story.append(PageBreak())

    # The Bottom Line
    story.append(Paragraph("The Bottom Line", styles['Heading1']))
    story.append(Spacer(1, 0.2 * inch))

    bottom_line = """The Token Proxy was built to answer: "How do we simplify the broker API for developers?" That's a valid question. But the answer should have been an SDK, not a mandatory infrastructure component. The proxy turned a developer experience question into an infrastructure deployment, and that single decision cascaded into every problem we found: no app entity, master key everywhere, audit blindness, infrastructure-as-registration.

The fix is straightforward: make apps real, give them their own credentials, let them talk to the broker directly (via SDK), and keep the proxy as an optional deployment for teams that want infrastructure-level resilience. The security model doesn't change at all — it gets stronger, because the master key stops spreading to every app deployment."""

    story.append(Paragraph(bottom_line, styles['Normal']))

    # Build PDF
    doc.build(story)

    print(f"PDF created successfully at: {output_path}")
    print(f"File size: {os.path.getsize(output_path) / 1024:.1f} KB")

    return output_path

if __name__ == "__main__":
    output = main()
    print(f"\nGenerated: {output}")
