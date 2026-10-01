// Tells osxd which window has the keyboard focus. Wayland keeps that from
// ordinary applications, so the shell answers on osxd's behalf over D-Bus.
import Gio from 'gi://Gio';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const IFACE = `
<node>
  <interface name="dev.nicklasfrahm.Osxd.Focus">
    <method name="WMClass">
      <arg type="s" direction="out" name="wm_class"/>
    </method>
  </interface>
</node>`;

export default class OsxdExtension extends Extension {
    enable() {
        this._dbus = Gio.DBusExportedObject.wrapJSObject(IFACE, this);
        this._dbus.export(Gio.DBus.session, '/dev/nicklasfrahm/Osxd');
    }

    disable() {
        this._dbus.unexport();
        this._dbus = null;
    }

    WMClass() {
        return global.display.focus_window?.get_wm_class() ?? '';
    }
}
